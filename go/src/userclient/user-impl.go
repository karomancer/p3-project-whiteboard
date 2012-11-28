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
	user      userproto.user
	homedir	string //path to home directory for Project Whiteboard files
	hostport  string
	midclient *midclient.midclient
	fileKeyMutex chan int
	fileKeyMap 	map[string]string //From local file path to its owner for quick searching
}

func iNewUserclient(myhostport string, homedir string) *Userclient {
	mclient, err := midclient.NewMidclient(myhostport)
	if err != nil {
		return nil
	}
	mutex := make(chan int, 1)
	mutex <- 1
	return &Userclient{Homedir: homedir, BuddyNode: mclient.Buddy, Hostport: myhostport, Midclient: mclient, FileKeyMutex: mutex, FileKeyMap: make(map[string]string)}
}

func (uc *Userclient) iCreateUser(args *userproto.CreateUserArgs, reply *userproto.CreateUserReply) error {
	if uc.user == nil {
		//User must log out first before creating another user
		reply.Status = EEXISTS
		return nil
	}
	_, exists := uc.midclient.Get(args.username)
	user := &User{Username: args.username, Password: args.Password, Email: args.Email, Classes: make(map[string]string)}
	if exists != nil {
		userjson, marshalErr := json.Marshal(user)
		if marshalErr != nil {
			return marshalErr
		}
		createErr := uc.midclient.Put(args.username, userjson)
		if createErr != nil {
			return createEr
		}
		uc.user = user
		reply.Status = userproto.OK
		return nil
	}
	reply.Status = userproto.EEXISTS
	return nil
}

//Walks directory structure to find all files and directories in each class
//and populates cache with their filepaths for easy storage access later
func (uc *Userclient) iWalkDirectoryStructure(keypath string, dir *storageproto.SyncFile) {
	keyend := strings.Split(classkey, ":")[1]
	filepath := uc.homedir + strings.Join(keyend.Split(keyend, "?"), "/")

	<- uc.fileKeyMutex 
	uc.fileKeyMap[filepath] = dir.Owner + "?" + keypath 
	uc.fileKeyMutex <- 1
	if dir.Files == nil { return }
	for path, file := dir.Files {
		uc.iWalkDirectoryStructure(path, file)
	}
}

func (uc *Userclient) iAuthenticateUser(args *userproto.AuthenticateUserArgs, reply *userproto.AuthenticateUserReply) error {
	userJSON, exists := uc.midclient.Get(args.username)
	if exists != nil {
		var user userproto.user
		jsonBytes := []byte(userJSON)
		unmarshalErr := json.Unmarshal(jsonBytes, &user)
		if unmarshalErr != nil { return unmarshalErr }
		if args.Password == user.Password {
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
	reply.Status = userproto.ENOSUCHUSER
	return nil
}

//To reflect added files in local
func (uc *Userclient) MonitorLocal() {

}


//To reflect changes in the server
func (uc *Userclient) MonitorServer() {
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
func (uc *Userclient) iSync() error {

}

//Can toggle sync (don't sync this file anymore) or sync it again!
func (uc *Userclient) iToggleSync(args *userproto.ToggleSyncArgs, reply *userproto.ToggleSyncReply) error {
	
}

func (uc *Userclient) iEditPermissions(args *userproto.EditPermissionsArgs, reply *userproto.EditPermissionsReply) error {
	
}
