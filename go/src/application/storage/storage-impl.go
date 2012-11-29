package storage

type StorageServer struct {
}

func iNewStorageServer(buddy string, portnum int, nodeid uint32) *StorageServer {
	return nil
}

//Returns portnumbers of nodes in skip list (1/2, 1/4, 1/8, ...) in order
func (ss *StorageServer) iGetSkipList() []int {
	return []int{}
}

func (ss *StorageServer) iRearrange(newnode string) error {
	return nil
}
