package storage

// The internal implementation of your storage server.
// Note:  This file does *not* provide a 'main' interface.  It is used
// by the 'storageserver' main function we have provided you, which
// will call storageimpl.NewStorageserver(...).
//
// Must implemement NewStorageserver and the RPC-able functions
// defined in the storagerpc StorageInterface interface.

//TO DO
//check each key if we are on the right server (rehash and check against our id, and other servers too since we have a list of all servers)
//leaseTimer
//other shits
//like figure out what a server is supposed to print and stuff when it joins and things

import (
	crand "crypto/rand"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/big"
	"math/rand"
	"net/rpc"
	"protos/storageproto"
	"strconv"
	"strings"
	"time"
)

type Storageserver struct {
	nodeid    uint32
	portnum   int
	isMaster  bool
	numNodes  int
	nodeList  []storageproto.Node
	nodeListM chan int
	nodeMap   map[storageproto.Node]uint32 //
	nodeMapM  chan int

	connMap  map[string]*rpc.Client //
	connMapM chan int

	valMap  map[string]string
	valMapM chan int

	srpc *Storageserver
}

func reallySeedTheDamnRNG() {
	randint, _ := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	rand.Seed(randint.Int64())
}

func (ss *Storageserver) GarbageCollector() {

}

func iNewStorageserver(buddy string, portnum int, nodeid uint32) *Storageserver {

	// //fmt.Println("called new storage server")

	// ss := &Storageserver{}

	// //if no nodeid is provided, choose one randomly
	// if nodeid == 0 {
	// 	reallySeedTheDamnRNG()
	// 	ss.nodeid = rand.Uint32()
	// } else {
	// 	//otherwise just take the one they gave you
	// 	ss.nodeid = nodeid
	// }

	// ss.portnum = portnum

	// ss.nodeList = []storageproto.Node{}
	// ss.nodeListM = make(chan int, 1)
	// ss.nodeListM <- 1

	// ss.nodeMap = make(map[storageproto.Node]int)
	// ss.nodeMapM = make(chan int, 1)
	// ss.nodeMapM <- 1

	// ss.connMap = make(map[string]*rpc.Client)
	// ss.connMapM = make(chan int, 1)
	// ss.connMapM <- 1

	// ss.valMap = make(map[string]string)
	// ss.valMapM = make(chan int, 1)
	// ss.valMapM <- 1

	// ss.numNodes = 0

	// for err != nil {
	// 	//keep retrying until we can actually conenct
	// 	//(Master may not have started yet)
	// 	masterClient, err = rpc.DialHTTP("tcp", master)
	// 	//fmt.Println("Trying to connect to master...")
	// 	time.Sleep(time.Duration(3) * time.Second)
	// }

	// //set up args for registering ourselves
	// info := storageproto.Node{HostPort: "localhost:" + strconv.Itoa(portnum), NodeID: ss.nodeid}
	// args := storageproto.RegisterArgs{ServerInfo: info}
	// reply := storageproto.RegisterReply{}

	// for err != nil || reply.Ready != true {
	// 	//call register on the master node with our info as the args. Kinda weird
	// 	err = masterClient.Call("StorageRPC.Register", &args, &reply)
	// 	//keep retrying until all things are registered
	// 	//fmt.Println("Trying to register with master...")
	// 	time.Sleep(time.Duration(3) * time.Second)
	// }

	// //gotta still set up some other shits
	// //like get list of servers from reply maybe?
	// //spec is pretty vague...
	// <-ss.nodeListM
	// //fmt.Println("Aquired nodeList lock NewStorageserver")
	// ss.nodeList = reply.Servers
	// log.Println("Successfully joined storage node cluster.")
	// slist := ""
	// for i := 0; i < len(ss.nodeList); i++ {
	// 	res := fmt.Sprintf("{localhost:%v %v}", ss.portnum, ss.nodeid)
	// 	slist += res
	// 	if i < len(ss.nodeList)-1 {
	// 		slist += " "
	// 	}
	// }
	// log.Printf("Server List: [%s]", slist)
	// ss.nodeListM <- 1
	// //fmt.Println("released nodeList lock NewStorageserver")

	// //non master doesn't keep a node map cause fuck you

	// ss.srpc = storagerpc.NewStorageRPC(ss)
	// rpc.Register(ss.srpc)
	// go ss.GarbageCollector()

	// //fmt.Println("started new server")
	// /*fmt.Println(storageproto.Node{HostPort: "localhost:" + strconv.Itoa(portnum), NodeID: ss.nodeid})
	// fmt.Printf("master? %v\n", ss.isMaster)
	// fmt.Printf("numnodes? %v\n", ss.numNodes)*/

	// return ss
}

// Non-master servers to the master
func (ss *Storageserver) RegisterServer(args *storageproto.RegisterArgs, reply *storageproto.RegisterReply) error {
	//called on master by other servers
	//first check if that server is alreayd in our map

	//fmt.Println("called register server")

	<-ss.nodeMapM
	//fmt.Println("aquired nodeMap lock RegisterServer")
	_, ok := ss.nodeMap[args.ServerInfo]
	//if not we have to add it to the map and to the list
	if ok != true {
		//put it in the list
		<-ss.nodeListM
		//fmt.Println("aquired nodeList lock RegisterServer")
		ss.nodeList = append(ss.nodeList, args.ServerInfo)
		ss.nodeListM <- 1
		//fmt.Println("release nodeList lock RegisterServer")
		//put it in the map w/ it's index in the list just cause whatever bro
		//map is just easy way to check for duplicates anyway
		ss.nodeMap[args.ServerInfo] = len(ss.nodeList)
	}

	//check to see if all nodes have registered
	<-ss.nodeListM
	//fmt.Println("aquired nodeList lock RegisterServer")
	if len(ss.nodeList) == ss.numNodes {
		//if so we are ready
		reply.Ready = true
		log.Println("Successfully joined storage node cluster.")
		slist := ""
		for i := 0; i < len(ss.nodeList); i++ {
			res := fmt.Sprintf("{localhost:%v %v}", ss.portnum, ss.nodeid)
			slist += res
			if i < len(ss.nodeList)-1 {
				slist += " "
			}
		}
		log.Printf("Server List: [%s]", slist)
	} else {
		//if not we aren't ready
		reply.Ready = false
	}

	//send back the list of servers anyway
	reply.Servers = ss.nodeList

	//unlock everything
	ss.nodeListM <- 1
	//fmt.Println("released nodeList lock RegisterServer")
	ss.nodeMapM <- 1
	//fmt.Println("released nodeMap lock RegisterServer")
	//NOTE: having these two mutexes may cause weird problems, might want to look into just having one mutex that is used for both the 
	//node list and the node map since they are baiscally the same thing anyway.

	//fmt.Println(reply.Servers)
	//fmt.Printf("ready? %v\n", reply.Ready)

	return nil
}

func (ss *Storageserver) GetServers(args *storageproto.GetServersArgs, reply *storageproto.RegisterReply) error {
	//this is what libstore calls on the master to get a list of all the servers
	//if the lenght of the nodeList is the number of nodes then we return ready and the list of nodes
	//otherwise we return false for ready and the list of nodes we have so far
	//fmt.Println("called get servers")
	<-ss.nodeListM
	//fmt.Println("aquried nodelist lock GetServers")
	//check to see if all nodes have registered
	if len(ss.nodeList) == ss.numNodes {
		//if so we are ready
		//fmt.Println("we are ready")
		reply.Ready = true
	} else {
		//if not we aren't ready
		reply.Ready = false
	}

	//send back the list of servers anyway
	reply.Servers = ss.nodeList

	ss.nodeListM <- 1
	return nil
}

func Storehash(key string) uint32 {
	hasher := fnv.New32()
	hasher.Write([]byte(key))
	return hasher.Sum32()
}

func (ss *Storageserver) checkServer(key string) bool {
	userclass := strings.Split(key, "?")[0]
	keyid := Storehash(precolon)

	if keyid > ss.nodeid {
		//but we might have wraparound!
		greaterThanAll := true
		for i := 0; i < len(ss.nodeList); i++ {
			if keyid < ss.nodeList[i].NodeID {
				greaterThanAll = false
			}
		}

		// if the key does need to be wrapped around, we need to make sure
		// it goes to the right server still, so we need to make sure our node
		// has the min node id, otherwise it's not the right one
		if greaterThanAll == true {
			lessThanAll := true
			for i := 0; i < len(ss.nodeList); i++ {
				if ss.nodeid > ss.nodeList[i].NodeID {
					lessThanAll = false
				}
			}

			//if it's not the least node id we have the wrong server
			if lessThanAll == false {
				return false
			} else {
				//otherwise we good
				return true
			}
		}

		// if the key doesn't need to be wrapped around, it's just in the wrong node
		return false
	}

	for i := 0; i < len(ss.nodeList); i++ {
		if keyid <= ss.nodeList[i].NodeID && ss.nodeList[i].NodeID < ss.nodeid {
			return false
		}
	}

	return true

}

// RPC-able interfaces, bridged via StorageRPC.
// These should do something! :-)

func (ss *Storageserver) Get(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	rightServer := ss.checkServer(args.Key)

	if rightServer == false {
		reply.Status = storageproto.EWRONGSERVER
		return nil
	}

	<-ss.valMapM
	val, ok := ss.valMap[args.Key]
	if ok != true {
		reply.Status = storageproto.EKEYNOTFOUND
		reply.Value = ""
	} else {
		reply.Status = storageproto.OK
		reply.Value = val
	}
	ss.valMapM <- 1

	return nil
}

func (ss *Storageserver) Put(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	rightServer := ss.checkServer(args.Key)

	if rightServer == false {
		reply.Status = storageproto.EWRONGSERVER
		return nil
	}

	<-ss.valMapM
	ss.valMap[args.Key] = args.Value
	ss.valMapM <- 1

	reply.Status = storageproto.OK
	return nil
}

func (ss *Storageserver) Delete(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	rightServer := ss.checkServer(args.Key)

	if rightServer == false {
		reply.Status = storageproto.EWRONGSERVER
		return nil
	}

	<-ss.valMapM
	delete(ss.listMap, args.Key)
	ss.valMapM <- 1

	reply.Status = storageproto.OK
	return nil
}
