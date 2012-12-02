package storageproto

//reply constants
const (
	OK = iota
	EWRONGSERVER
	EKEYNOTFOUND
)

const (
	DEFAULT = iota //Only syncs for that user to server (not to anyone else)
	CLASS
	LECTURES
	ASSIGNMENTS
	SUBMISSION
	OFFICEHOURS
	OTHER //for other class files or folders from the prof, or if students have a collaborative hw etc.
)

//constants for permissions
const (
	READ  = iota //can read only
	WRITE        //can read and write
	COPY         //r/w a copy of the file: when students can write to their own copy of the file which then syncs to the teacher but does not overwrite the teacher's original file
	NONE         //signals that we should remove user from permission list
)

type Node struct {
	HostPort string
	NodeID   uint32
}

// //Are both Directory and SyncFile structs needed?
// //Also the Class directory struct
// type Directory struct {
// 	Owner       *userproto.User
// 	Dir         *os.File
// 	DirInfo     *os.FileInfo
// 	Files       []*SyncFile    //consider (*File) Readdirnames that reads names from within directory
// 	Permissions map[string]int //map of users w/ access to directory 
// 	Synced      bool
// }

type GetArgs struct {
	Key    string
	Client string
}

type GetReply struct {
	Status   int
	JSONFile string
}

type PutArgs struct {
	Key      string
	JSONFile string
}

type PutReply struct {
	Status int
}

type RegisterArgs struct {
	ServerInfo Node
}

// RegisterReply is sent in response to both Register and GetServers
type RegisterReply struct {
	Servers []Node
}
