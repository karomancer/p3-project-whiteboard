//Definition of API for midclient calls

package midclient

import (
	"storage"
	"os"
)


func NewMidClient(myhostport string) (*Midclient, error) {
	return iNewMidClient(server, myhostport)
}

func (mc *Midclient) CreateUser(name string, password string, email string, usertype string) error {
	return mc.iCreateUser(name, password, email, usertype)
}

func (mc *Midclient) AddClass(name string, class string) error {
	return mc.iAddClass(name, class)
}

//Returns marshalled:
// * users
// * File descriptors (files)
// * File descriptors (directories)
func (mc *Midclient) Get(key string) (string, error) {
	return mc.iGet(key)
}

//Put covers basic Syncing as well
//Put a file at a certain key
//Automatically adds to the directory list it belongs in

//Can also be used to make directories
//(FileMode) IsDir can tell if its a directory...in FileInfo
func (mc *Midclient) Put(key, filetype int, data string) error {
	return mc.iPut(key, filetype, file)
}

//User deleted file locally; remove from repository
//Should we make another one if the user deletes from the repository?
//(e.g. professor removes a file, should that sync with user?)
func (mc *Midclient) DeleteFile(file *storage.SyncFile) error {
	return mc.iDeleteFile(file)
}

//Add/Remove sync   
//(any future changes of this particular file will not be synced to the server)
//may be used if the user is running out of space
func (mc *Midclient) ToggleSync(file string) error {
	return mc.iToggleSync(file)
}

//Get list of files in the current directory (only keys are needed)
//Separate Directory struct may be needed for meta data about directory

//consider (*File) Readdirnames that reads names from within directory
//Necessary considering how a dir is just a file?
func (mc *Midclient) GetDir(key string) ([]string, error) {
	return mc.iGetDir(key)
}

//Maybe a string instead of a filemode...
func (mc *Midclient) AddPermissions(filekey string, mode *os.FileMode, users string[]) error {
	return mc.iAddPersmissions(filekey, mode, users)
}

func (mc *Midclient) RemovePermissions(filekey string, mode *os.FileMode, users string[]) error {
	return mc.iRemovePersmissions(filekey, mode, users)
}


//For Tier 3:
//AddtoQueue
//RemoveFromQueue


// Partitioning:  Defined here so that all implementations
// use the same mechanism.
//Same as P2
func Storehash(key string) uint32 {
	hasher := fnv.New32()
	hasher.Write([]byte(key))
	return hasher.Sum32()
}



\