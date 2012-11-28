package userclient

import (
	"encoding/json"
	"midclient"
	"os"
	"strings"
	"path/filepath"
)

type userclient struct {
	User userproto.User
	Homedir	string //path to home directory for Project Whiteboard files
	Hostport  string
	Midclient *midclient.Midclient
	FileKeyMutex chan int
	FileKeyMap 	map[string]string //From local file path to its owner for quick searching
}

func iNewUserclient(myhostport string, homedir string) *Userclient {
	mclient, err := midclient.NewMidclient(myhostport)
	if err != nil {
		return nil
	}
	mutex := make(chan int, 1)
	mutex <- 1
	return &Userclient{Homedir: homedir, Hostport: myhostport, Midclient: mclient, FileKeyMutex: mutex, FileKeyMap: make(map[string]string)}
}

func (uc *Userclient) CreateUser(args *userproto.CreateUserArgs, reply *userproto.CreateUserReply) error {
	if uc.User == nil {
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
func (uc *Userclient) iWalkDirectoryStructure(keypath string, dir *storageproto.SyncFile) {
	keyend := strings.Split(classkey, ":")[1]
	filepath := uc.Homedir + strings.Join(keyend.Split(keyend, "?"), "/")

	<- uc.FileKeyMutex 
	uc.FileKeyMap[filepath] = dir.Owner + "?" + keypath 
	uc.FileKeyMutex <- 1
	if dir.Files == nil { return }
	for path, file := dir.Files {
		uc.iWalkDirectoryStructure(path, file)
	}
}

func (uc *Userclient) iAuthenticateUser(args *userproto.AuthenticateUserArgs, reply *userproto.AuthenticateUserReply) error {
	userJSON, exists := uc.Midclient.Get(args.Username)
	//if they do...
	if exists != nil {
		var user userproto.User
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
			uc.User = user
			for classkey, _ := uc.User.Classes {

				//It doesn't make sense for it to not exist
				classJSON, _ := uc.Midclient.Get(classkey)
				
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

func (uc *Userclient) iMonitor() {
	for {
		time.Sleep(10 * time.Second)
		<- uc.FileKeyMutex
		cache, _ := uc.FileKeyMap
		uc.FileKeyMutex <- 1
		for _, filepath := cache {
			//TODO - Karina will finish this in the morning :X
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

//can change permissions on a file, but only if you are the owner of a file. On front end will be triggered by "share this file" and can choose whether they can read or write to it.
func (uc *Userclient) iEditPermissions(args *userproto.AddPermissionsArgs, reply *userproto.AddPermissionsReply) error {
	//if args.permission = nil then we are removing people from the permission list
	//also gotta check if we are adding a user to the permission list if the user actually exists
	//otherwise if the user already exists on the list we just change the permission to the new value
	//may want to change permissions to better reflect the scaling of them or something gah
}
