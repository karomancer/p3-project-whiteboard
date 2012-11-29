//Definition of API for midclient calls

package midclient

import (
	"fmt"
	"net/rpc"
	"os"
	"packages/lsplog"
	"protos/storageproto"
	"strings"
	// "storage"
)

type Midclient struct {
	hostport        string
	buddy           *rpc.Client
	connectionCache map[string]*rpc.Client
	connCacheMutex  chan int
}

//Needs to find buddy node, but for simplicity start 
//by providing buddy node
func iNewMidClient(server string, myhostport string) (*Midclient, error) {
	mc := &Midclient{hostport: myhostport}

	// Create RPC connection to storage server
	buddy, dialErr := rpc.DialHTTP("tcp", server)
	if dialErr != nil {
		fmt.Printf("Could not connect to server %s, returning nil\n", server)
		return nil, dialErr
	}
	mc.buddy = buddy

	mc.connectionCache = make(map[string]*rpc.Client)
	mc.connCacheMutex = make(chan int, 1)
	mc.connCacheMutex <- 1

	//Cache RPC?
	return mc, nil
}

//figures out which server the key hashes to by jumping aroudn the server ring
func (mc *Midclient) getNode(key string) (*rpc.Client, error) {
	//Store by either user (user has no prefix) or by class@[directory/file] 
	class := strings.Split(key, "?")[0]
	keyid := Storehash(class)

	//because as of yet unused....
	keyid = keyid

	<-mc.connCacheMutex
	node, ok := mc.connectionCache[class]
	mc.connCacheMutex <- 1

	if ok == true {
		return node, nil
	}

	//else, find node (on server side: skip list)?
	//****** IMPLEMENT HERE! ********

	//rpc call to buddy node which then asks around for the right server and then returns that guy

	return nil, nil

}

//Returns marshalled:
// * users
// * File descriptors (files)
// * File descriptors (directories)
func (mc *Midclient) iGet(key string) (string, error) {
	// Store based on file/directory owner

	//find out which node to contact
	node, getServerErr := mc.getNode(key)
	if getServerErr != nil {
		fmt.Fprintf(os.Stderr, " error in get node\n")
		return "", getServerErr
	}

	//set up the args with the key
	args := &storageproto.GetArgs{key, mc.hostport}
	//set up the reply.....
	var reply storageproto.GetReply
	//Get that stuff
	err := node.Call("StorageRPC.Get", args, &reply)
	if err != nil {
		return "", err
	}

	//return that stuff
	return reply.JSONFile, nil
}

//Put covers basic Syncing as well
//Put a file at a certain key
//Automatically adds to the directory list it belongs in

//Can also be used to make directories
//and users
//(FileMode) IsDir can tell if its a directory...in FileInfo
func (mc *Midclient) iPut(key string, data string) error {
	//figure out who we gotta talk to 
	node, getServerErr := mc.getNode(key)
	if getServerErr != nil {
		fmt.Fprintf(os.Stderr, " error in get node\n")
		return getServerErr
	}

	//set up args and reply
	args := &storageproto.PutArgs{key, data}
	var reply storageproto.PutReply

	//actually put the stuff
	putErr := node.Call("StorageRPC.Put", args, &reply)
	if putErr != nil {
		return putErr
	}
	if reply.Status != storageproto.OK {
		return lsplog.MakeErr("Put failed: Storage error")
	}
	//Sucess!
	return nil
}

//*** Initially, don't implement this shit ***/
//User deleted locally; remove from repository
//Should we make another one if the user deletes from the repository?
//(e.g. professor removes a file, should that sync with user?)
func (mc *Midclient) iDeleteFile(key string) error {
	return nil
}

//This is probably just actually a call to Get/Put from the user client so should be removed
//Add/Remove sync   
//(any future changes of this particular file will not be synced to the server)
//may be used if the user is running out of space
func (mc *Midclient) iToggleSync(key string) error {
	return nil

}
