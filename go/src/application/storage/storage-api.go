package storage

//Heavenly package to use:
// http://golang.org/pkg/os
// import (
// 	"os"
// )

func NewStorageServer(buddy string, portnum int, nodeid uint32) *StorageServer {
	return iNewStorageServer(buddy, portnum, nodeid)
}

//Returns portnumbers of nodes in skip list (1/2, 1/4, 1/8, ...) in order
func (ss *StorageServer) GetSkipList() []int {
	return ss.GetSkipList()
}

func (ss *StorageServer) Rearrange(newnode string) error {
	return ss.iRearrange(newnode)
}
