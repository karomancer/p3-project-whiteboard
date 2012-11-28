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

type Userclient struct {
	User userproto.User
	Homedir	string //path to home directory for Project Whiteboard files
	Hostport  string
	Midclient *midclient.Midclient
	FileKeyMutex chan int
	FileKeyMap 	map[string]string //From local file path to its full storage key for quick searching
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
	//check to see if Username already exists
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
	userJSON, exists := uc.midclient.Get(args.Username)
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
				unmarshalErr = json.Unmarshal(classBytes, &class)
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
						_ = json.Unmarshal(syncBytes, &syncFile)
						// if unmarshalErr == nil { fmt.Println("Unmarshal error.\n") }
							
							//If local was updated more recently, put local to server
							if fileinfo.ModTime().After(syncFile.FileInfo.ModTime()) {
								if marshalErr == nil {
									//If has permissions, overwrite.
									//Additionally, if student in class, take old sync file as original
									if syncFile.Permissions[uc.user.Username] == storageproto.WRITE {
										if uc.user.Classes[syncFile.Class] == STUDENT {

										}

										syncFile.File = file
										syncFile.FileInfo = fileinfo

										fileJSON, marshalErr := json.Marshal(syncFile)

										createErr := uc.midclient.Put(filekey, fileJSON)
										if createErr != nil {
											fmt.Println("Creation error!\n")
										}	 
										
									} 
									else { //If not, make new file in storage with owner as this student
										syncFile.File = file
										syncFile.FileInfo = fileinfo
										syncFile.Owner = uc.user.Username
										
										fileJSON, _ := json.Marshal(syncFile)
										//Put new file in storage server
										newkey := uc.user.Username + ":" + strings.Split(filekey, ":")[1]
										createErr := uc.midclient.Put(newkey, fileJSON) 
										if createErr != nil {
											fmt.Println("Creation error!\n")
										}
										//Associate with class
										classkey := syncFile.Class
										classJSON, _ := uc.midclient.Get(classkey)

										var classFile storageproto.SyncFile
										classBytes := []byte(classJSON)
										json.Unmarshal(classBytes, &classFile)

										append(classFile.Files, syncFile)
										classJSONEdit, _ := json.Marshal(classFile)
										uc.midclient.Put(classkey)
									}
								}
									 	
							}	
							else {		//else, copy server to local
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

	//get the current directory because we can only edit permissions of a file when we are in it's directory
	currDir, wdErr := os.Getwd()
	if wdErr != nil { return wdErr }
	//strip the path down to only the path after the WhiteBoard file, since the rest is not consistant computer to computer
	paths := strings.SplitAfterN(currDir, "WhiteBoard", -1)
	//create the filepath to the actula file 
	filepath := paths[1] + "/" + args.Filename
	//find out the key from the FileKeyMap
	key, exists := uc.FileKeyMap[filepath]
	if exists != true {
		reply.Status = userproto.ENOSUCHFILE
		return nil
	}
	//actually get the current permissions info from the server
	//LATER: can also cache this info if we are acessing it frequently to reduce RPC calls
	//chances are in real life however that this won't be acessed very frequently from any particular user
	//so may be safe to ignore that case
	jfile, getErr := us.Midclient.Get(key)
	if getErr != nil { return getErr }
	//unmarshal that shit
	var file storageproto.SyncFile
	fileBytes := []byte(jfile)
	unmarshalErr = json.Unmarshal(fileBytes, &file)
	if unmarshalErr != nil { return unmarshalErr }

	//get the current permissions
	//we don't need to lock anything while changing the permissions because only the owner of a particular file can change the permissions, 
	//which means that there won't be any instance where two people are changing the same permissions at once
	permissions := file.permissions

	//go through all the users
	for i := 0; i < args.Users; i++ {
		_, exists := permissions[args.Users[i]]
		//if the dude already exists then just change the permissions
		if exists == true {
			//if the permissions is NONE then we just remove the dude from the list
			if args.Permission == storageproto.NONE {
				delete(permissions, args.Users[i])
			} else {
				permissions[args.Users[i]] = args.Permission
			}
		} else {
			//otherwise we have to check if the dude is a valid dude
			_, exists := uc.Midclient.Get(args.Users[i])
			//if he is then we can add him to the list
			if exists != nil {
				//if the permission is NONE then just don't add him
				if args.Permission != storageproto.NONE {
					permissions[args.Users[i]] = args.Permission
				}
			}
		}
	}
	//we are done so the permissions are changed and we need to update the server
	file.permissions = permissions
	filejson, marshalErr := json.Marshal(file)
	if marshalErr != nil { return marshalErr }

	err := uc.Midclient.Put(key, filejson)
	if err != nil { return err }

	reply.Status = userproto.OK

	return nil
}


