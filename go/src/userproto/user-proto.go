package userproto

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
	Filename string
}

type ToggleSyncReply struct {
	Status int
}

type EditPermissionsArgs struct {
	Filename   string
	Users      []User
	Permission int
}

type EditPermissionsReply struct {
	Status int
}
