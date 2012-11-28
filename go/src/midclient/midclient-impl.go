//Definition of API for midclient calls

package midclient

import (
	"net/rpc"
	"os"
	"storage"
)

type Midclient struct {
	Hostport        string
	Buddy           *rpc.Client
	ConnectionCache map[string]*rpc.Client
	ConnCacheMutex  chan int
}

//Needs to find buddy node, but for simplicity start 
//by providing buddy node
func iNewMidClient(server string, myhostport string) (*Midclient, error) {
	mc := &Midclient{Hostport: myhostport}

	// Create RPC connection to storage server
	buddy, dialErr := rpc.DialHTTP("tcp", server)
	if dialErr != nil {
		fmt.Printf("Could not connect to server %s, returning nil\n", server)
		return nil, dialErr
	}
	mc.Buddy = buddy

	mc.ConnectionCache = make(map[string]*rpc.Client)
	mc.ConnCacheMutex = make(chan int, 1)
	mc.ConnCacheMutex <- 1

	//Cache RPC?
	return mc, nil
}

func (mc *Midclient) getNode(key string) (*rpc.Client, error) {
	//Store by either user (user has no prefix) or by class@[directory/file] 
	class := strings.Split(key, "?")[0]
	keyid := Storehash(class)

	<-mc.ConnCacheMutex
	node, ok := mc.ConnectionCache[class]
	mc.ConnCacheMutex <- 1

	if ok == true {
		return node, nil
	}

	//else, find node (on server side: skip list)?
	//****** IMPLEMENT HERE! ********
}

//Returns marshalled:
// * users
// * File descriptors (files)
// * File descriptors (directories)
func (mc *Midclient) iGet(key string) (string, error) {
	// Store based on file/directory owner
	node, getServerErr := mc.getNode(key)
	if getServerErr != nil {
		fmt.Fprintf(os.Stderr, " error in get node\n")
		return "", err
	}

	args := &storageproto.GetArgs{key, mc.Hostport}
	err = node.Call("StorageRPC.Get", args, &reply)
	if err != nil {
		return "", err
	}

	return reply.JSONFile, nil
}

//Put covers basic Syncing as well
//Put a file at a certain key
//Automatically adds to the directory list it belongs in

//Can also be used to make directories
//and users
//(FileMode) IsDir can tell if its a directory...in FileInfo
func (mc *Midclient) iPut(key string, data string) error {
	node, getServerErr := mc.getNode(key)
	if getServerErr != nil {
		fmt.Fprintf(os.Stderr, " error in get node\n")
		return "", getServerErr
	}

	args := &storageproto.PutArgs{key, data}
	var reply storageproto.PutReply

	putErr := node.Call("StorageRPC.Put", args, &reply)
	if putErr != nil {
		return putErr
	}
	if reply.Status != storageproto.OK {
		return log.MakeErr("Put failed: Storage error")
	}
	return nil
}
