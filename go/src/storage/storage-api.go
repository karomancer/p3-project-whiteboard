package storage

//Heavenly package to use:
// http://golang.org/pkg/os
import (
	"os"
)

const (
	DEFAULT=0 //Only syncs for that user to server (not to anyone else)
	LECTURES=1
	ASSIGNMENTS=2
	SUBMISSION=3
	OFFICEHOURS=4
	OTHER=5
)


type User struct {
	Name string
	Password string
	Email string
	Type string
}

//Keep a list of Syncfiles on the midclient side to remmeber which files are to be
//synced and which are not
type SyncFile struct {
	File *os.File 
	FileInfo *os.FileInfo
	//Default permissions if in a preset folder, else can be set for custom folder types
	Permissions map[string]*os.FileMode 
	Synced bool
}


//Are both Directory and SyncFile structs needed?
type Directory struct {
	Dir *os.File
	DirInfo *os.FileInfo
	//consider (*File) Readdirnames that reads names from within directory
	Files []*SyncFile
	Permissions map[string]*os.FileMode
	Synced bool
}

func NewStorageServer(buddy string, portnum int, nodeid uint32) *StorageServer {
	return iNewStorageServer(buddy, portnum, nodeid)
}

//Returns portnumbers of nodes in skip list (1/2, 1/4, 1/8, ...) in order
func (ss *Storageserver) GetSkipList() []int { 
	return ss.GetSkipList()
}

func (ss *Storageserver) Rearrange(newnode string) error {
	return ss.iRearrange(newnode)
}

//ETC ETC


