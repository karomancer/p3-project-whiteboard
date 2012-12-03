package storage

//Heavenly package to use:
// http://golang.org/pkg/os
import (
	"protos/storageproto"
)

func NewStorageServer(buddy string, portnum int, nodeid uint32) *Storageserver {
	return iNewStorageserver(buddy, portnum, nodeid)
}

func (ss *Storageserver) Get(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	return ss.iGet(args, reply)
}

func (ss *Storageserver) Put(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	return ss.iPut(args, reply)
}

func (ss *Storageserver) Delete(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	return ss.iDelete(args, reply)
}

//Returns portnumbers of nodes in skip list (1/2, 1/4, 1/8, ...) in order
// func (ss *Storageserver) GetSkipList() []int {
// 	return ss.GetSkipList()
// }

// func (ss *Storageserver) Rearrange(newnode string) error {
// 	return ss.iRearrange(newnode)
// }
