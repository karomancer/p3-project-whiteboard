package userproto

import {
	"os"
}
//**USER TYPES
const (
	STUDENT    = 0 //Only syncs for that user to server (not to anyone else)
	INSTRUCTOR = 1
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
	Classes  map[string]string //Class to role in class (instructor or student?)
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

type PutFileArgs struct {
	Username  string
	Directoryname string
	File  *os.File //? or File descriptor?
}

type PutFileReply struct {
	Status string
}

type GetFileArgs struct {
	Username string
	Filename string
}

type GetFileReply struct {
	Status  int
	File *os.File
}

type GetDirectoryArgs struct { // Used for both GetTribbles and GetTribblesBySubscription
	Username string
	Directoryname string
}

type GetDirectoryReply struct {
	Status   int
	Files []*os.File
}

type AddPermissionsArgs struct {
	Username string
	Permissions *os.FileMode 
	Filename *os.File
}

type AddPermissionsReply struct {
	Status int
}

type RemovePermissionsArgs struct {
	Username string
	Permissions *os.FileMode
	Filename *os.File
}

type RemovePermissionsReply struct {
	Status int
}