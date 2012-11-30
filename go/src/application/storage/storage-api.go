package storage

//Heavenly package to use:
// http://golang.org/pkg/os
// import (
// 	"os"
// )

func NewStorageServer(buddy string, portnum int, nodeid uint32) *Storageserver {
	return iNewStorageserver(buddy, portnum, nodeid)
}

//Returns portnumbers of nodes in skip list (1/2, 1/4, 1/8, ...) in order
// func (ss *Storageserver) GetSkipList() []int {
// 	return ss.GetSkipList()
// }

// func (ss *Storageserver) Rearrange(newnode string) error {
// 	return ss.iRearrange(newnode)
// }
