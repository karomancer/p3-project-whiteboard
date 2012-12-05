package userclient

import (
	"application/midclient"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"packages/github.com/howeyc/fsnotify"
	"protos/storageproto"
	"protos/userproto"
	"strconv"
	"strings"
	"time"
)

type KeyPermissions struct {
	Key         string
	Permissions map[string]int
}

type Userclient struct {
	user          *userproto.User
	homedir       string //path to home directory for Project Whiteboard files
	hostport      string
	midclient     *midclient.Midclient
	watcher       *fsnotify.Watcher
	wdChangeMutex chan int //working directory changes

	permkeyFileMutex chan int
	permkeyFile      *os.File

	classMutex chan int
	fileCheck  chan int

	fileKeyMutex chan int
	fileKeyMap   map[string]KeyPermissions //From local file path to its full storage key for quick searching
}

func iNewUserClient(myhostport string, buddy string, homedir string) *Userclient {
	//WARNING!!!! PUT TEMP STRING AS ARG BECAUSE NO IDEA HOW WE ARE SUPPOSED TO KNOW WHAT
	//SERVER TO CONNECT TO FROM THIS SIDE....
	mclient, err := midclient.NewMidClient(buddy, myhostport)
	if err != nil {
		return nil
	}
	wdMutex := make(chan int, 1)
	wdMutex <- 1

	classMutex := make(chan int, 1)
	classMutex <- 1

	mutex := make(chan int, 1)
	mutex <- 1
	filecheck := make(chan int)
	user := &userproto.User{}
	return &Userclient{classMutex: classMutex, wdChangeMutex: wdMutex, homedir: homedir, fileCheck: filecheck, hostport: myhostport, midclient: mclient, fileKeyMutex: mutex, fileKeyMap: make(map[string]KeyPermissions), user: user}
}

func (uc *Userclient) iCreateUser(args *userproto.CreateUserArgs, reply *userproto.CreateUserReply) error {
	if uc.user.Username != "" {
		//User must log out first before creating another user
		reply.Status = userproto.EEXISTS
		return nil
	}
	//check to see if Username already exists
	getUser, _ := uc.midclient.Get(args.Username, "")
	//if it doesn't we are good to go
	if getUser == "" {
		//make new user
		//LATER: should actually hash the password before we store it.....
		user := &userproto.User{Username: args.Username, Password: args.Password, Email: args.Email, Classes: []string{}}
		//marshal it
		userjson, marshalErr := json.Marshal(user)
		if marshalErr != nil {
			return marshalErr
		}
		//send it to the server
		createErr := uc.midclient.Put(args.Username, string(userjson), "")
		if createErr != nil {
			return createErr
		}
		//set the current user of the session to the one we just created
		uc.user = user

		dirErr := os.MkdirAll(uc.homedir, os.ModeDir)
		if dirErr != nil {
			return dirErr
		}

		watcher, watchErr := fsnotify.NewWatcher()
		if watchErr != nil {
			log.Fatal(watchErr)
		}
		uc.watcher = watcher
		go uc.iMonitorLocalChanges()
		watchErr = watcher.Watch(uc.homedir)

		if watchErr != nil {
			log.Fatal(watchErr)
		}

		cdErr := os.Chdir(uc.homedir)
		if cdErr != nil {
			return cdErr
		}

		//Set up hidden file with permissions and file/key cache
		permFile, permErr := os.Create(".permkey")

		permFile.Chmod(os.FileMode(444))
		if permErr != nil {
			return permErr
		}

		permKeyMutex := make(chan int, 1)
		permKeyMutex <- 1
		uc.permkeyFileMutex = permKeyMutex
		uc.permkeyFile = permFile

		reply.Status = userproto.OK
		return nil
	}
	//otherwise the name already exists and we must chose another name
	reply.Status = userproto.EEXISTS
	return nil
}

//Walks directory structure to find all files and directories in each class
//and populates cache with their filepaths for easy storage access later
func (uc *Userclient) iConstructFileKeyMap() {
	<-uc.permkeyFileMutex
	reader := bufio.NewReader(uc.permkeyFile)
	for {
		line, _, readErr := reader.ReadLine()
		if readErr != nil {
			break
		}
		//Read in line by line and parse to maps
		lineArray := strings.Split(string(line), " ")

		//First lets do permissions
		//starts at 2 because 0 = filepath, 1= key
		permMap := make(map[string]int)
		for i := 2; i < len(lineArray); i++ {
			lineArraySquared := strings.Split(lineArray[i], ":")
			username := lineArraySquared[0]

			perm, convErr := strconv.Atoi(lineArraySquared[1])
			if convErr == nil {
				permMap[username] = perm
			}
		}

		//Then do map
		<-uc.fileKeyMutex
		uc.fileKeyMap[lineArray[0]] = KeyPermissions{lineArray[1], permMap}
		uc.fileKeyMutex <- 1
	}
	fmt.Printf("%v\n", uc.fileKeyMap)
	uc.permkeyFileMutex <- 1
}

func (uc *Userclient) iAuthenticateUser(args *userproto.AuthenticateUserArgs, reply *userproto.AuthenticateUserReply) error {
	userJSON, exists := uc.midclient.Get(args.Username, "")
	if exists == nil {
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
			file, readErr := os.Open(uc.homedir)
			if readErr != nil {
				dirErr := os.MkdirAll(uc.homedir, os.ModeDir)
				if dirErr != nil {
					return dirErr
				}
			} else {
				file.Close()
			}

		} else {
			reply.Status = userproto.WRONGPASSWORD
			return nil
		}
		permMutex := make(chan int, 1)
		permMutex <- 1
		uc.permkeyFileMutex = permMutex

		permkeyFile, permkeyErr := os.Open(uc.homedir + "/.permkey")
		uc.permkeyFile = permkeyFile

		//if permkey file exists...
		//construct table
		if permkeyErr == nil {
			fmt.Println("Exists")
			uc.iConstructFileKeyMap()
		} else { //else make it!
			fmt.Println("Make permkey")
			permFile, permErr := os.Create(".permkey")
			permFile.Chmod(os.FileMode(444))
			if permErr != nil {
				return permErr
			}
		}

		watcher, watchErr := fsnotify.NewWatcher()
		if watchErr != nil {
			log.Fatal(watchErr)
		}
		uc.watcher = watcher
		go uc.iMonitorLocalChanges()

		watchErr = watcher.Watch(uc.homedir)

		if watchErr != nil {
			log.Fatal(watchErr)
		}

		cdErr := os.Chdir(uc.homedir)
		if cdErr != nil {
			return cdErr
		}

		go uc.iInitialFileCheck() // updates outdated file on local on login

		return nil
	}
	//if there isn't a user by that name just return error
	reply.Status = userproto.ENOSUCHUSER
	return nil
}

//To reflect added files in server
func (uc *Userclient) iMonitorLocalChanges() {
	for {
		select {
		case ev := <-uc.watcher.Event:
			if ev.IsCreate() {
				//Making the key at which to store the Syncfile.
				//First, take the path (given by watcher.Event) and parse out the /'s to replace with ?'s
				patharray := strings.Split(ev.Name, "/")
				foundWhite := false
				//if for some resaons Whiteboard is in the path, take it out and all prefix
				var i int
				for i = 0; foundWhite == false && i < len(patharray); i++ {
					if patharray[i] == "whiteboard" {
						foundWhite = true
					}
				}
				//If it wasn't...reset i
				if foundWhite == false {
					i = 0
				}
				// The key is user:class?path to file (delimited by ?'s')
				intermed := strings.Join(patharray[i:], "?")
				class := patharray[i]
				if class == ".permkey" {
					break
				}
				fmt.Println("Class ", class)
				key := fmt.Sprintf("%v:%v", uc.user.Username, intermed)

				//Changing directory madness starts here
				//cos apparently you have to be ni that directory (parameter is filename, not filepath)
				<-uc.wdChangeMutex
				pwd, _ := os.Getwd()
				fmt.Printf("WD %v for %v\n", pwd, ev.Name)
				for j := i; j < len(patharray)-1; j++ {
					cdErr := os.Chdir(patharray[j])
					if cdErr != nil {
						fmt.Println("Couldn't cd into ", patharray[j])
						break
					}
				}

				file, fileErr := os.Open(patharray[len(patharray)-1])
				if fileErr != nil {
					fmt.Println("Fail ", patharray[len(patharray)-1])
					break
				}

				existSyncFile := uc.iGet(key)
				var permissions map[string]int

				//if it already exists, we're just copying it over from server (don't overwrite server)
				if existSyncFile == nil { //if it doesn't exist
					//Make the sync file to store
					fi, statErr := file.Stat()
					if statErr != nil {
						break
					}

					permissions = make(map[string]int)
					permissions[uc.user.Username] = storageproto.WRITE
					var files []string
					if fi.IsDir() {
						files = []string{}
					} else {
						files = nil
					}

					//Finding content of file
					buffer := bytes.NewBuffer(make([]byte, 0))
					<-uc.permkeyFileMutex
					reader := bufio.NewReader(file)
					for {
						line, _, readErr := reader.ReadLine()
						if readErr != nil {
							break
						}
						buffer.Write(line)

					}
					uc.permkeyFileMutex <- 1

					syncFile := &storageproto.SyncFile{Owner: uc.user, Class: class, UpdateTime: time.Now().Nanosecond(), Contents: buffer.Bytes(), Files: files, Permissions: permissions, Synced: true}

					//Give to midclient
					uc.iPush(key, syncFile)
					//Get parent to add to Files list
					parentfilepath := uc.homedir + "/" + strings.Join(patharray[:(len(patharray)-1)], "/")
					<-uc.fileKeyMutex
					//parent has to be in map...otherwise this would make literally 0 sense
					parentkeyperm, _ := uc.fileKeyMap[parentfilepath]
					fmt.Println("My parent: ", parentfilepath)
					uc.fileKeyMutex <- 1
					parentSync := uc.iGet(parentkeyperm.Key)
					if parentSync != nil {
						parentSync.Files = append(parentSync.Files, key)
						uc.iPush(parentkeyperm.Key, parentSync)
					}
				} else {
					permissions = existSyncFile.Permissions
				}
				//cd back to whiteboard/
				for j := i; j < len(patharray)-1; j++ {
					os.Chdir("..")
				}
				file.Close()

				//also add to file
				<-uc.permkeyFileMutex
				toFile := fmt.Sprintf("%v %v %v:%v\n", ev.Name, key, uc.user.Username, storageproto.WRITE)
				_, writeErr := uc.permkeyFile.WriteString(toFile)
				if writeErr != nil {
					fmt.Println("Write error!")
				}
				uc.permkeyFileMutex <- 1

				//Hash for easy access later
				uc.fileKeyMap[ev.Name] = KeyPermissions{key, permissions}

				uc.wdChangeMutex <- 1
				fmt.Printf("Create Event: %v\n", ev)
			} else if ev.IsModify() {

			} else if ev.IsDelete() {
				fmt.Printf("Delete Event: %v\n", ev)
			} else if ev.IsRename() {
				fmt.Printf("Rename Event: %v\n", ev)
			}
		case err := <-uc.watcher.Error:
			fmt.Println("File error: ", err)
		}
	}
}

func (uc *Userclient) iMakeClass(args *userproto.MakeClassArgs, reply *userproto.MakeClassReply) error {
	file, fileErr := os.Open(args.Classname)
	fmt.Printf("%v\n", file)
	//if the file doesn't already exist, make class!
	if fileErr != nil {
		<-uc.wdChangeMutex
		//make the class directory
		dirErr := os.Mkdir(args.Classname, os.ModeDir)
		if dirErr != nil {
			return dirErr
		}
		//CD into that directory to make files within it
		//Lock change mutex so another go routine doesn't put something in the wrong spot
		watchErr := uc.watcher.Watch(args.Classname)
		if watchErr != nil {
			fmt.Println("Watch error cos fuck you")
		}
		//Here are the default types of directories...
		dirArray := []string{"Lectures", "Assignments", "Other"}
		for i := 0; i < len(dirArray); i++ {
			os.Chdir(args.Classname)
			dirErr = os.Mkdir(dirArray[i], os.ModeDir)
			//Go back to whiteboard home in wd
			//don't ask, its stupid
			os.Chdir("..")
			uc.watcher.Watch(args.Classname + "/" + dirArray[i])
			if dirErr != nil {
				return dirErr
			}
		}
		//add class to class map on user
		//no need for mutex because client can only run this sequentially
		uc.wdChangeMutex <- 1

		<-uc.classMutex
		uc.user.Classes = append(uc.user.Classes, uc.user.Username+":"+args.Classname)
		uc.classMutex <- 1

		//marshal it
		userjson, marshalErr := json.Marshal(uc.user)
		if marshalErr != nil {
			return marshalErr
		}
		//send it to the server
		createErr := uc.midclient.Put(uc.user.Username, string(userjson), "")
		if createErr != nil {
			return createErr
		}

		reply.Status = userproto.OK

	} else {
		file.Close()
		reply.Status = userproto.EEXISTS
	}
	return nil
}

// func (uc *Userclient) iMakeAssignment(args *userproto.MakeAssignmentArgs, reply *userproto.MakeAssignmentReply) error {
// }

func (uc *Userclient) iPush(key string, file *storageproto.SyncFile) {
	if file.Synced == true {
		fileJSON, marshalErr := json.Marshal(file)
		if marshalErr != nil {
			log.Fatal("Marshal error\n")
		}
		createErr := uc.midclient.Put(key, string(fileJSON), uc.user.Username)
		if createErr != nil {
			fmt.Println("Server put error!\n")
		}
	}
}

func (uc *Userclient) iGet(key string) *storageproto.SyncFile {
	JSON, getErr := uc.midclient.Get(key, uc.user.Username)
	if getErr != nil {
		fmt.Println("GetErr! Does not exist!\n")
		return nil
	}
	var file storageproto.SyncFile
	fileBytes := []byte(JSON)
	unmarshalErr := json.Unmarshal(fileBytes, &file)
	if unmarshalErr != nil {
		fmt.Println("Unmarshal error!\n")
		return nil
	}
	return &file
}

func (uc *Userclient) iWalkDownClass(key string) {
	syncFile := uc.iGet(key)
	if syncFile != nil {
		keyend := strings.Split(key, ":")[1]
		pathArray := strings.Split(keyend, "?")
		filename := strings.Join(pathArray, "/")

		_, openErr := os.Open(filename)
		if openErr != nil { //then it doesn't exist (path error)
			<-uc.wdChangeMutex
			//if directory, make a directory
			//either way, make the monitor local changes function take care of most things
			if syncFile.Files != nil {
				dirErr := os.MkdirAll(filename, os.ModeDir)
				fmt.Println("Walking filename to watch: ", filename)
				uc.watcher.Watch(filename)
				if dirErr != nil {
					return
				}
			} else { //else a file
				fmt.Println("Is file!")
				for i := 0; i < len(pathArray)-1; i++ {
					cdErr := os.Chdir(pathArray[i])
					if cdErr != nil {
						return
					}
				}
				writeFile, fileErr := os.Create(filename)
				if fileErr != nil {
					return
				}
				//Finding content of file
				writeFile.Write(syncFile.Contents)
				writeFile.Close()
				for i := 0; i < len(pathArray)-1; i++ {
					cdErr := os.Chdir("..")
					if cdErr != nil {
						return
					}
				}
			}

			uc.wdChangeMutex <- 1
			//make the class directory

		} else { //if it does exist, check with current file to see if newly updated

		}
		fmt.Printf("Key: %v has Files: %v", key, syncFile.Files)
		for i := 0; i < len(syncFile.Files); i++ {
			uc.iWalkDownClass(syncFile.Files[i])
		}

	}
}

//To reflect changes in the local
func (uc *Userclient) iInitialFileCheck() {
	//Check out all files in user classes list
	classes := uc.user.Classes

	for i := 0; i < len(classes); i++ {
		uc.iWalkDownClass(classes[i])
	}

	// cache := uc.fileKeyMap
	// uc.fileKeyMutex <- 1
	// //for every node in our cache, check to see if it got updated
	// for filepath, keyperm := range cache {
	// 	filekey := keyperm.Key
	// 	file, fileOpenErr := os.Open(filepath) //Opens
	// 	if fileOpenErr != nil {
	// 		fmt.Printf("File %v open error\n", filepath)
	// 		//call Midclient Delete function
	// 	} else {
	// 		fileinfo, statErr := file.Stat()
	// 		if statErr == nil {
	// 			syncfileJSON, getErr := uc.midclient.Get(filekey, uc.user.Username)
	// 			if getErr == nil {
	// 				//If no errors, get file off of server to compare
	// 				var syncFile *storageproto.SyncFile
	// 				syncBytes := []byte(syncfileJSON)
	// 				_ = json.Unmarshal(syncBytes, &syncFile)
	// 				// if unmarshalErr == nil { fmt.Println("Unmarshal error.\n") }

	// 				//If local was updated more recently, put local to server
	// 				syncFileInfo, _ := syncFile.File.Stat()
	// 				if fileinfo.ModTime().After(syncFileInfo.ModTime()) {
	// 					//If has permissions, overwrite.
	// 					//Additionally, if student in class, take old sync file as original
	// 					if syncFile.Permissions[uc.user.Username] == storageproto.WRITE {
	// 						syncFile.File = file
	// 						uc.iPush(filekey, syncFile)
	// 					} else { //If not, make new file in storage with owner as this student
	// 						syncFile.File = file
	// 						syncFile.Owner = uc.user
	// 						//Put new file in storage server
	// 						newkey := uc.user.Username + ":" + strings.Split(filekey, ":")[1]
	// 						uc.iPush(newkey, syncFile)
	// 						//Associate with class
	// 						classkey := syncFile.Class

	// 						classFile := uc.iGet(classkey)
	// 						if classFile.Files == nil {
	// 							classFile.Files = []string{}
	// 						}
	// 						classFile.Files = append(classFile.Files, newkey)
	// 						uc.iPush(classkey, classFile)
	// 					}

	// 				} else { //else, copy server to local...that is if server is newer
	// 					file.Truncate(fileinfo.Size())
	// 					var content []byte
	// 					_, readErr := syncFile.File.Read(content)
	// 					if readErr == nil {
	// 						file.Write(content)
	// 					}
	// 				}

	// 				//TODO: If changed but didn't have permission to: 
	// 				//make new file for original (called <filename>_original or the like)

	// 			} else { //if file doesn't already exist
	// 				dirarray := strings.Split(filekey, "?")
	// 				class := dirarray[0]
	// 				permissions := make(map[string]int)
	// 				permissions[uc.user.Username] = storageproto.WRITE

	// 				newowner := uc.user.Username + ":" + strings.Split(class, ":")[1]
	// 				newkey := newowner + strings.Split(filekey, ":")[1]

	// 				syncFile := &storageproto.SyncFile{Owner: uc.user, Class: class, File: file, Permissions: permissions, Synced: true}
	// 				uc.iPush(newkey, syncFile)

	// 				parentkey := strings.Join(dirarray[0:len(dirarray)-2], "?")

	// 				parentFile := uc.iGet(parentkey)
	// 				if parentFile == nil {
	// 					parentFd, openErr := os.Open(uc.homedir + strings.Join(dirarray[1:len(dirarray)-2], "/"))
	// 					if openErr != nil {
	// 						fmt.Println("Open error!\n")
	// 						file.Close()
	// 						return
	// 					}
	// 					parentFile = &storageproto.SyncFile{Owner: uc.user, Class: class, File: parentFd, Files: nil, Permissions: permissions, Synced: true}
	// 				}
	// 				if parentFile.Files == nil {
	// 					parentFile.Files = []string{}
	// 				}
	// 				parentFile.Files = append(parentFile.Files, filekey)
	// 				uc.iPush(parentkey, syncFile)
	// 			}
	// 		}
	// 	}
	// 	file.Close()
	// }

}

//Things get pushed to the user automatically, but in case it's acting funny the user can also 
//manually ask for a sync
func (uc *Userclient) iSync() error {
	return nil
}

//Can toggle sync (don't sync this file anymore) or sync it again!
func (uc *Userclient) iToggleSync(args *userproto.ToggleSyncArgs, reply *userproto.ToggleSyncReply) error {
	<-uc.fileKeyMutex
	filekey := uc.fileKeyMap[args.Filepath].Key
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
	keyperm, exists := uc.fileKeyMap[filepath]
	if exists != true {
		reply.Status = userproto.ENOSUCHFILE
		return nil
	}
	key := keyperm.Key
	//actually get the current permissions info from the server
	//LATER: can also cache this info if we are acessing it frequently to reduce RPC calls
	//chances are in real life however that this won't be acessed very frequently from any particular user
	//so may be safe to ignore that case
	jfile, getErr := uc.midclient.Get(key, uc.user.Username)
	if getErr != nil {
		return getErr
	}
	//unmarshal that shit
	var file storageproto.SyncFile
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
			_, exists := uc.midclient.Get(args.Users[i], "")
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

	err := uc.midclient.Put(key, string(filejson), uc.user.Username)
	if err != nil {
		return err
	}

	reply.Status = userproto.OK

	return nil
}

func (uc *Userclient) iIsLoggedIn() string {
	if (uc.user != nil) && (uc.user != &userproto.User{}) {
		return uc.user.Username
	}
	return ""
}
