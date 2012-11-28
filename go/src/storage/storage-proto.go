package storageproto

const (
	DEFAULT = itoa //Only syncs for that user to server (not to anyone else)
	CLASS
	LECTURES
	ASSIGNMENTS
	SUBMISSION
	OFFICEHOURS
	OTHER //for other class files or folders from the prof, or if students have a collaborative hw etc.
)


//constants for permissions
const (
	READ = itoa //can read only
	WRITE //can read and write
	COPY //r/w a copy of the file: when students can write to their own copy of the file which then syncs to the teacher but does not overwrite the teacher's original file
)

type Node struct {
	HostPort string
	NodeID   uint32
}

//Keep a list of Syncfiles on the midclient side to remmeber which files are to be
//synced and which are not
type SyncFile struct {
	Owner       *userproto.User
	File        *os.File
	FileInfo    *os.FileInfo
	Permissions map[string]int //Default permissions if in a preset folder, else can be set for custom folder types
	Synced      bool
}

//Are both Directory and SyncFile structs needed?
type Directory struct {
	Owner       *userproto.User
	Dir         *os.File
	DirInfo     *os.FileInfo
	Files       []*SyncFile //consider (*File) Readdirnames that reads names from within directory
	Permissions map[string]int //map of users w/ access to directory 
	Synced      bool
}

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
	Ready   bool
	Servers []Node
}
