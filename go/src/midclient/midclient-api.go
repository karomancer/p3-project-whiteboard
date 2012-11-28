//Definition of API for midclient calls

package midclient

//Needs to find buddy node
func NewMidClient(server string, myhostport string) (*Midclient, error) {
	return iNewMidClient(server, myhostport)
}

//Returns marshalled:
// * users
// * Classes
// * File descriptors (files)
// * File descriptors (directories)
func (mc *Midclient) Get(key string) (string, error) {
	return mc.iGet(key)
}

//Put covers basic Syncing as well
//Put a file at a certain key
//Automatically adds to the directory list it belongs in

//Can also be used to make directories
//and users
//(FileMode) IsDir can tell if its a directory...in FileInfo
func (mc *Midclient) Put(key, data string) error {
	return mc.iPut(key, file)
}

//*** Initially, don't implement this shit ***/
//User deleted locally; remove from repository
//Should we make another one if the user deletes from the repository?
//(e.g. professor removes a file, should that sync with user?)
func (mc *Midclient) Delete(key string) error {
	return mc.iDeleteFile(file)
}

//This is probably just actually a call to Get/Put from the user client so should be removed
//Add/Remove sync   
//(any future changes of this particular file will not be synced to the server)
//may be used if the user is running out of space
func (mc *Midclient) ToggleSync(key string) error {
	return mc.iToggleSync(file)

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
