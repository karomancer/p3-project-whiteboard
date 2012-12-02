package userproto

import (
	"os"
)

//**USER TYPES
const (
	STUDENT = iota //Only syncs for that user to server (not to anyone else)
	INSTRUCTOR
)

// Status codes
const (
	OK = iota
	NOTLOGGEDIN
	ENOSUCHUSER
	WRONGPASSWORD
	ENOSUCHTARGETUSER
	ENOSUCHCLASS
	ENOSUCHFILE
	ENOSUCHDIRECTORY
	EEXISTS
)

type User struct {
	Username string
	Password string
	Email    string
	UserType int
	Classes  map[string]int //Class to role in class (instructor or student?)
}

//Keep a list of Syncfiles on the midclient side to remmeber which files are to be
//synced and which are not
type SyncFile struct {
	Owner       *User
	Class       string         //classkey owner:class
	File        *os.File       // if dir, can use "Readdir(0) will return all FileInfos associated with this dir"
	Files       []string       // nil if not dir, else keys of files
	Permissions map[string]int //Default permissions if in a preset folder, else can be set for custom folder types
	Synced      bool
}

type CreateUserArgs struct {
	Username string
	Password string
	Email    string
}

type CreateUserReply struct {
	Status int
}

type AuthenticateUserArgs struct {
	Username string
	Password string
}

type AuthenticateUserReply struct {
	Status int
}

type ToggleSyncArgs struct {
	Filepath string
}

type ToggleSyncReply struct {
	Status int
}

type EditPermissionsArgs struct {
	Filepath   string
	Permission int
	Users      []string
}

type EditPermissionsReply struct {
	Status int
}
