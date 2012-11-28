package userclient

import (
	"encoding/json"
	"midclient"
	"os"
	"fmt"
	"strings"
	"path/filepath"
	"time"
)

type userclient struct {
	User userproto.User
	Homedir	string //path to home directory for Project Whiteboard files
	Hostport  string
	Midclient *midclient.Midclient
	FileKeyMutex chan int
	FileKeyMap 	map[string]string //From local file path to its owner for quick searching
}

func iNewuserclient(myhostport string, homedir string) *userclient {
	mclient, err := midclient.NewMidclient(myhostport)
	if err != nil {
		return nil
	}
	mutex := make(chan int, 1)
	mutex <- 1
	return &userclient{Homedir: homedir, Hostport: myhostport, Midclient: mclient, FileKeyMutex: mutex, FileKeyMap: make(map[string]string)}
}

func (uc *userclient) iCreateUser(args *userproto.CreateUserArgs, reply *userproto.CreateUserReply) error {
	if uc.user == nil {
		//User must log out first before creating another user
		reply.Status = EEXISTS
		return nil
	}
	//check to see if username already exists
	_, exists := uc.Midclient.Get(args.Username)
	//if it doesn't we are good to go
	if exists == nil {
		//make new user
		//LATER: should actually hash the password before we store it.....
		user := &User{Username: args.Username, Password: args.Password, Email: args.Email, Classes: make(map[string]string)}
		//marshal it
		userjson, marshalErr := json.Marshal(user)
		if marshalErr != nil {
			return marshalErr
		}
		//send it to the server
		createErr := uc.Midclient.Put(args.Username, userjson)
		if createErr != nil {
			return createEr
		}
		//set the current user of the session to the one we just created
		uc.User = user
		reply.Status = userproto.OK
		return nil
	}
	//otherwise the name already exists and we must chose another name
	reply.Status = userproto.EEXISTS
	return nil
}

//Walks directory structure to find all files and directories in each class
//and populates cache with their filepaths for easy storage access later
func (uc *userclient) iWalkDirectoryStructure(keypath string, dir *storageproto.SyncFile) {
	keyend := strings.Split(keypath, ":")[1]
	filepath := uc.homedir + strings.Join(keyend.Split(keyend, "?"), "/")

	<- uc.fileKeyMutex 
	uc.fileKeyMap[filepath] = dir.Owner + "?" + keypath 
	uc.fileKeyMutex <- 1
	if dir.Files == nil { return }
	for path, file := dir.Files {
		uc.iWalkDirectoryStructure(path, file)
	}
}

func (uc *userclient) iAuthenticateUser(args *userproto.AuthenticateUserArgs, reply *userproto.AuthenticateUserReply) error {
	userJSON, exists := uc.midclient.Get(args.username)
	if exists != nil {
		var user userproto.user
		jsonBytes := []byte(userJSON)
		//unmarshall the data
		unmarshalErr := json.Unmarshal(jsonBytes, &user)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		//check if the passwords match
		//LATER: should actually just hash the password and then check the hashes
		if args.Password == user.Password {
			//if they do it's all good and we are logged in
			reply.Status = userproto.OK
			//Get user data for temporary session
			uc.user = user
			for classkey, _ := range uc.user.Classes {
				//It doesn't make sense for it to not exist
				classJSON, _ := uc.midclient.Get(classkey)
				
				var class storageproto.SyncFile
				classBytes := []byte(classJSON)
				unmarshalErr = json.Unmarshal(jsonBytes, &class)
				if unmarshalErr != nil { return unmarshalErr }

				uc.iWalkDirectoryStructure(classkey, class)
			
			}
 		} else {
			reply.Status = userproto.WRONGPASSWORD
		}
		return nil
	}
	//if there isn't a user by that name just return error
	reply.Status = userproto.ENOSUCHUSER
	return nil
}

//To reflect added files in local
func (uc *userclient) MonitorLocal() {

}


//To reflect changes in the server
func (uc *userclient) iMonitorServer() {
	for {
		time.Sleep(10 * time.Second)
		<- uc.fileKeyMutex
		cache, _ := uc.fileKeyMap
		uc.fileKeyMutex <- 1
		for filepath, filekey := cache {
			file, fileOpenErr := os.Open(filepath) //Opens
			if fileOpenErr != nil {
				fmt.Printf("File %v open error\n", filepath)
				//call Midclient Delete function
			} else {
				fileinfo, statErr := file.Stat()
				if statErr == nil {
					syncfileJSON, getErr := uc.midclient.Get(filekey)
					if getErr == nil {
						//If no errors, get file off of server to compare
						var syncFile storageproto.SyncFile
						syncBytes := []byte(syncfileJSON)
						unmarshalErr = json.Unmarshal(syncBytes, &syncFile)
						if unmarshalErr == nil { fmt.Println("Unmarshal error.\n") }
							
							//If local was updated more recently, put local to server
							if fileinfo.ModTime().After(syncFile.FileInfo.ModTime()) {
								syncFile.File = file
								syncFile.FileInfo = fileinfo

								fileJSON, marshalErr := json.Marshal(syncFile)
								if marshalErr == nil {
									//If has permissions, overwrite.
									//Additionally, if student in class, take old sync file as original
									if syncFile.Permissions[uc.user.username] == storageproto.WRITE {
										createErr := uc.midclient.Put(filekey, fileJSON)
										if createErr != nil {
											fmt.Println("Creation error!\n")
										}	 
										
									} else { //If not, make new file in storage with owner as this student
										newkey := uc.user.username + ":" + strings.Split(filekey, ":")[1]
										createErr := uc.midclient.Put(newkey, fileJSON) 
										if createErr != nil {
											fmt.Println("Creation error!\n")
										}
									}
								}


									 	
							}	 else {		//else, copy server to local
								file.Truncate(fileinfo.Size())
								var content [syncFile.FileInfo.Size()]byte
								_, readErr := syncFile.Read(content)
								if readErr == nil {
									file.Write(content)
								}
						}

						//TODO: If changed but didn't have permission to: 
						//make new file for original

					} 
				}
			}
		}
	}
}

//Things get pushed to the user automatically, but in case it's acting funny the user can also 
//manually ask for a sync
func (uc *userclient) iSync() error {

}

//Can toggle sync (don't sync this file anymore) or sync it again!
func (uc *userclient) iToggleSync(args *userproto.ToggleSyncArgs, reply *userproto.ToggleSyncReply) error {
	
}

//can change permissions on a file, but only if you are the owner of a file. On front end will be triggered by "share this file" and can choose whether they can read or write to it.
func (uc *userclient) iEditPermissions(args *userproto.AddPermissionsArgs, reply *userproto.AddPermissionsReply) error {
	//if args.permission = nil then we are removing people from the permission list
	//also gotta check if we are adding a user to the permission list if the user actually exists
	//otherwise if the user already exists on the list we just change the permission to the new value
	//may want to change permissions to better reflect the scaling of them or something gah
}
