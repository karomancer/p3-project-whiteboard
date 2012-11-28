package userclient

import (
	"encoding/json"
	"midclient"
	"os"
	"strings"
	"path/filepath"
)

type Userclient struct {
	User      userproto.User
	Homedir	string //path to home directory for Project Whiteboard files
	Hostport  string
	Midclient *midclient.Midclient
	FileKeyMutex chan int
	FileKeyMap 	map[string]string //From local file path to its owner for quick searching
}

func NewUserclient(myhostport string, homedir string) *Userclient {
	mclient, err := midclient.NewMidclient(myhostport)
	if err != nil {
		return nil
	}
	mutex := make(chan int, 1)
	mutex <- 1
	return &Userclient{Homedir: homedir, BuddyNode: mclient.Buddy, Hostport: myhostport, Midclient: mclient, FileKeyMutex: mutex, FileKeyMap: make(map[string]string)}
}

func (uc *Userclient) CreateUser(args *userproto.CreateUserArgs, reply *userproto.CreateUserReply) error {
	if uc.User == nil {
		//User must log out first before creating another user
		reply.Status = EEXISTS
		return nil
	}
	_, exists := uc.Midclient.Get(args.Username)
	user := &User{Username: args.Username, Password: args.Password, Email: args.Email, Classes: make(map[string]string)}
	if exists != nil {
		userjson, marshalErr := json.Marshal(user)
		if marshalErr != nil {
			return marshalErr
		}
		createErr := uc.Midclient.Put(args.Username, userjson)
		if createErr != nil {
			return createEr
		}
		uc.User = user
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
	filepath := uc.Homedir + strings.Join(keyend.Split(keyend, "?"), "/")

	<- uc.FileKeyMutex 
	uc.FileKeyMap[filepath] = dir.Owner + "?" + keypath 
	uc.FileKeyMutex <- 1
	if dir.Files == nil { return }
	for path, file := dir.Files {
		uc.iWalkDirectoryStructure(path, file)
	}
}

func (uc *Userclient) AuthenticateUser(args *userproto.AuthenticateUserArgs, reply *userproto.AuthenticateUserReply) error {
	userJSON, exists := uc.Midclient.Get(args.Username)
	if exists != nil {
		var user userproto.User
		jsonBytes := []byte(userJSON)
		unmarshalErr := json.Unmarshal(jsonBytes, &user)
		if unmarshalErr != nil { return unmarshalErr }
		if args.Password == user.Password {
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
	reply.Status = userproto.ENOSUCHUSER
	return nil
}

func (uc *Userclient) Monitor() {
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
func (uc *Userclient) Sync() error {

}

//Can toggle sync (don't sync this file anymore) or sync it again!
func (uc *Userclient) ToggleSync(args *userproto.ToggleSyncArgs, reply *userproto.ToggleSyncReply) error {
	
}

func (uc *Userclient) EditPermissions(args *userproto.EditPermissionsArgs, reply *userproto.EditPermissionsReply) error {
	
}
