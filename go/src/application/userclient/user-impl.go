package userclient

import (
	"application/midclient"
	"encoding/json"
	"fmt"
	"os"
	"packages/github.com/howeyc/fsnotify"
	// "path/filepath"
	"log"
	"protos/storageproto"
	"protos/userproto"
	"strings"
)

type Userclient struct {
	user         *userproto.User
	homedir      string //path to home directory for Project Whiteboard files
	hostport     string
	midclient    *midclient.Midclient
	fileCheck    chan int
	fileKeyMutex chan int
	fileKeyMap   map[string]string //From local file path to its full storage key for quick searching
}

func iNewUserClient(myhostport string, homedir string) *Userclient {
	//WARNING!!!! PUT TEMP STRING AS ARG BECAUSE NO IDEA HOW WE ARE SUPPOSED TO KNOW WHAT
	//SERVER TO CONNECT TO FROM THIS SIDE....
	mclient, err := midclient.NewMidClient("server", myhostport)
	if err != nil {
		return nil
	}
	mutex := make(chan int, 1)
	mutex <- 1
	filecheck := make(chan int)
	return &Userclient{homedir: homedir, fileCheck: filecheck, hostport: myhostport, midclient: mclient, fileKeyMutex: mutex, fileKeyMap: make(map[string]string)}
}

func (uc *Userclient) iCreateUser(args *userproto.CreateUserArgs, reply *userproto.CreateUserReply) error {
	if uc.user.Username != "" {
		//User must log out first before creating another user
		reply.Status = userproto.EEXISTS
		return nil
	}
	//check to see if Username already exists
	_, exists := uc.midclient.Get(args.Username)
	//if it doesn't we are good to go
	if exists == nil {
		//make new user
		//LATER: should actually hash the password before we store it.....
		user := &userproto.User{Username: args.Username, Password: args.Password, Email: args.Email, Classes: make(map[string]int)}
		//marshal it
		userjson, marshalErr := json.Marshal(user)
		if marshalErr != nil {
			return marshalErr
		}
		//send it to the server
		createErr := uc.midclient.Put(args.Username, string(userjson))
		if createErr != nil {
			return createErr
		}
		//set the current user of the session to the one we just created
		uc.user = user
		reply.Status = userproto.OK
		return nil
	}
	//otherwise the name already exists and we must chose another name
	reply.Status = userproto.EEXISTS
	return nil
}

//Walks directory structure to find all files and directories in each class
//and populates cache with their filepaths for easy storage access later
func (uc *Userclient) iWalkDirectoryStructure(keypath string) {
	keyend := strings.Split(keypath, ":")[1]
	filepath := uc.homedir + strings.Join(strings.Split(keyend, "?"), "/")

	fileJSON, _ := uc.midclient.Get(keypath)
	var file userproto.SyncFile
	fileBytes := []byte(fileJSON)
	json.Unmarshal(fileBytes, &file)

	<-uc.fileKeyMutex
	uc.fileKeyMap[filepath] = file.Owner.Username + "?" + keypath
	uc.fileKeyMutex <- 1
	if file.Files == nil {
		uc.fileCheck <- 1
		return
	}
	for i := 0; i < len(file.Files); i++ {
		uc.iWalkDirectoryStructure(file.Files[i])
	}
}

func (uc *Userclient) iAuthenticateUser(args *userproto.AuthenticateUserArgs, reply *userproto.AuthenticateUserReply) error {
	userJSON, exists := uc.midclient.Get(args.Username)
	if exists != nil {
		var user *userproto.User
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

				var class userproto.SyncFile
				classBytes := []byte(classJSON)
				unmarshalErr = json.Unmarshal(classBytes, &class)
				if unmarshalErr != nil {
					return unmarshalErr
				}
				go uc.iWalkDirectoryStructure(classkey) // creates cache of available files

			}
		} else {
			reply.Status = userproto.WRONGPASSWORD
			return nil
		}
		watcher, watchErr := fsnotify.NewWatcher()
		if watchErr != nil {
			log.Fatal(watchErr)
		}

		go uc.iMonitorLocalChanges(watcher)
		watchErr = watcher.Watch(uc.homedir)

		if watchErr != nil {
			log.Fatal(watchErr)
		}

		go uc.iInitialFileCheck() // updates outdated file on local on login

		return nil
	}
	//if there isn't a user by that name just return error
	reply.Status = userproto.ENOSUCHUSER
	return nil
}

//To reflect added files in server
func (uc *Userclient) iMonitorLocalChanges(watcher *fsnotify.Watcher) {
	for {
		select {
		case ev := <-watcher.Event:
			fmt.Println("Event: ", ev)
		case err := <-watcher.Error:
			fmt.Println("File error: ", err)
		}
	}
}

func (uc *Userclient) iPush(key string, file *userproto.SyncFile) {
	if file.Synced == true {
		fileJSON, marshalErr := json.Marshal(file)
		if marshalErr != nil {
			log.Fatal("Marshal error\n")
		}
		createErr := uc.midclient.Put(key, string(fileJSON))
		if createErr != nil {
			fmt.Println("Server put error!\n")
		}
	}
}

func (uc *Userclient) iGet(key string) *userproto.SyncFile {
	JSON, getErr := uc.midclient.Get(key)
	if getErr != nil {
		fmt.Println("GetErr! Does not exist!\n")
		return nil
	}
	var file userproto.SyncFile
	fileBytes := []byte(JSON)
	unmarshalErr := json.Unmarshal(fileBytes, &file)
	if unmarshalErr != nil {
		fmt.Println("Unmarshal error!\n")
	}
	return &file
}

//To reflect changes in the local
func (uc *Userclient) iInitialFileCheck() {
	count := 0
	for count < len(uc.user.Classes) {
		count += (<-uc.fileCheck)
	}

	cache := uc.fileKeyMap
	uc.fileKeyMutex <- 1
	for filepath, filekey := range cache {
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
					var syncFile *userproto.SyncFile
					syncBytes := []byte(syncfileJSON)
					_ = json.Unmarshal(syncBytes, &syncFile)
					// if unmarshalErr == nil { fmt.Println("Unmarshal error.\n") }

					//If local was updated more recently, put local to server
					syncFileInfo, _ := syncFile.File.Stat()
					if fileinfo.ModTime().After(syncFileInfo.ModTime()) {
						//If has permissions, overwrite.
						//Additionally, if student in class, take old sync file as original
						if syncFile.Permissions[uc.user.Username] == storageproto.WRITE {
							syncFile.File = file
							uc.iPush(filekey, syncFile)
						} else { //If not, make new file in storage with owner as this student
							syncFile.File = file
							syncFile.Owner = uc.user
							//Put new file in storage server
							newkey := uc.user.Username + ":" + strings.Split(filekey, ":")[1]
							uc.iPush(newkey, syncFile)
							//Associate with class
							classkey := syncFile.Class

							classFile := uc.iGet(classkey)
							if classFile.Files == nil {
								classFile.Files = []string{}
							}
							classFile.Files = append(classFile.Files, newkey)
							uc.iPush(classkey, classFile)
						}

					} else { //else, copy server to local...that is if server is newer
						file.Truncate(fileinfo.Size())
						var content []byte
						_, readErr := syncFile.File.Read(content)
						if readErr == nil {
							file.Write(content)
						}
					}

					//TODO: If changed but didn't have permission to: 
					//make new file for original (called <filename>_original or the like)

				} else { //if file doesn't already exist
					dirarray := strings.Split(filekey, "?")
					class := dirarray[0]
					permissions := make(map[string]int)
					permissions[uc.user.Username] = storageproto.WRITE

					newowner := uc.user.Username + ":" + strings.Split(class, ":")[1]
					newkey := newowner + strings.Split(filekey, ":")[1]

					syncFile := &userproto.SyncFile{Owner: uc.user, Class: class, File: file, Permissions: permissions, Synced: true}
					uc.iPush(newkey, syncFile)

					parentkey := strings.Join(dirarray[0:len(dirarray)-2], "?")

					parentFile := uc.iGet(parentkey)
					if parentFile == nil {
						parentFd, openErr := os.Open(uc.homedir + strings.Join(dirarray[1:len(dirarray)-2], "/"))
						if openErr != nil {
							fmt.Println("Open error!\n")
							file.Close()
							return
						}
						parentFile = &userproto.SyncFile{Owner: uc.user, Class: class, File: parentFd, Files: []string{}, Permissions: permissions, Synced: true}
					}
					if parentFile.Files == nil {
						parentFile.Files = []string{}
					}
					parentFile.Files = append(parentFile.Files, filekey)
					uc.iPush(parentkey, syncFile)
				}
			}
		}
		file.Close()
	}

}

//Things get pushed to the user automatically, but in case it's acting funny the user can also 
//manually ask for a sync
func (uc *Userclient) iSync() error {
	return nil
}

//Can toggle sync (don't sync this file anymore) or sync it again!
func (uc *Userclient) iToggleSync(args *userproto.ToggleSyncArgs, reply *userproto.ToggleSyncReply) error {
	<-uc.fileKeyMutex
	filekey := uc.fileKeyMap[args.Filepath]
	uc.fileKeyMutex <- 1
	syncFile := uc.iGet(filekey)
	if syncFile == nil {
		log.Fatal("Unable to get file.")
	}
	syncFile.Synced = false
	uc.iPush(filekey, syncFile)
	return nil
}

//can change permissions on a file, but only if you are the owner of a file. On front end will be triggered by "share this file" and can choose whether they can read or write to it.
func (uc *Userclient) iEditPermissions(args *userproto.EditPermissionsArgs, reply *userproto.EditPermissionsReply) error {
	//if args.permission = nil then we are removing people from the permission list
	//also gotta check if we are adding a user to the permission list if the user actually exists
	//otherwise if the user already exists on the list we just change the permission to the new value
	//may want to change permissions to better reflect the scaling of them or something gah

	//get the current directory because we can only edit permissions of a file when we are in it's directory
	currDir, wdErr := os.Getwd()
	if wdErr != nil {
		return wdErr
	}
	//strip the path down to only the path after the WhiteBoard file, since the rest is not consistant computer to computer
	paths := strings.SplitAfterN(currDir, "WhiteBoard", -1)
	//create the filepath to the actula file 
	filepath := paths[1] + "/" + args.Filepath
	//find out the key from the FileKeyMap
	key, exists := uc.fileKeyMap[filepath]
	if exists != true {
		reply.Status = userproto.ENOSUCHFILE
		return nil
	}
	//actually get the current permissions info from the server
	//LATER: can also cache this info if we are acessing it frequently to reduce RPC calls
	//chances are in real life however that this won't be acessed very frequently from any particular user
	//so may be safe to ignore that case
	jfile, getErr := uc.midclient.Get(key)
	if getErr != nil {
		return getErr
	}
	//unmarshal that shit
	var file userproto.SyncFile
	fileBytes := []byte(jfile)
	unmarshalErr := json.Unmarshal(fileBytes, &file)
	if unmarshalErr != nil {
		return unmarshalErr
	}

	//get the current permissions
	//we don't need to lock anything while changing the permissions because only the owner of a particular file can change the permissions, 
	//which means that there won't be any instance where two people are changing the same permissions at once
	permissions := file.Permissions

	//go through all the users
	for i := 0; i < len(args.Users); i++ {
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
			_, exists := uc.midclient.Get(args.Users[i])
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
	file.Permissions = permissions
	filejson, marshalErr := json.Marshal(file)
	if marshalErr != nil {
		return marshalErr
	}

	err := uc.midclient.Put(key, string(filejson))
	if err != nil {
		return err
	}

	reply.Status = userproto.OK

	return nil
}
