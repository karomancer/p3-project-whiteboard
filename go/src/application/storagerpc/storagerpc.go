// This is a quick adapter to ensure that only the desired interfaces
// are exported, and to define the storage server interface.
//
// Do not modify this file for 15-440.
//
// Implement your changes in your own contrib/storageimpl code.
//

package storagerpc

import (
	"protos/storageproto"
)

type StorageInterface interface {
	RegisterServer(*storageproto.RegisterArgs, *storageproto.RegisterReply) error
	//GetServers(*storageproto.GetServersArgs, *storageproto.RegisterReply) error
	Get(*storageproto.GetArgs, *storageproto.GetReply) error
	Put(*storageproto.PutArgs, *storageproto.PutReply) error
	Delete(*storageproto.GetArgs, *storageproto.GetReply) error
	TransferData(*storageproto.TransferArgs, *storageproto.TransferReply) error
}

type StorageRPC struct {
	ss StorageInterface
}

func NewStorageRPC(ss StorageInterface) *StorageRPC {
	return &StorageRPC{ss}
}

func (srpc *StorageRPC) Get(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	return srpc.ss.Get(args, reply)
}

func (srpc *StorageRPC) Put(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	return srpc.ss.Put(args, reply)
}

func (srpc *StorageRPC) Delete(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	return srpc.ss.Delete(args, reply)
}

func (srpc *StorageRPC) Register(args *storageproto.RegisterArgs, reply *storageproto.RegisterReply) error {
	return srpc.ss.RegisterServer(args, reply)
}

func (srpc *StorageRPC) Transfer(args *storageproto.TransferArgs, reply *storageproto.TransferReply) error {
	return srpc.ss.TransferData(args, reply)
}

/*func (srpc *StorageRPC) GetServers(args *storageproto.GetServersArgs, reply *storageproto.RegisterReply) error {
	return srpc.ss.GetServers(args, reply)
}*/
