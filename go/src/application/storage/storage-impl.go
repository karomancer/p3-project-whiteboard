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
	"sort"
)

type Storageserver struct {
	nodeid    uint32
	portnum   int

	nodeList  []storageproto.Node //list of all other nodes and portnumbers, SORTED
	nodeListM chan int

	connMap  map[int]*rpc.Client //map from nodeID to connection for your skiplist
	connMapM chan int

	valMap  map[string]string //map of actual stuff we are storing... I think
	valMapM chan int

	srpc *Storageserver
}

func reallySeedTheDamnRNG() {
	randint, _ := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	rand.Seed(randint.Int64())
}

func (ss *Storageserver) GarbageCollector() {
	//do we need this since we don't have leases?
}

//set up sorting for the storage server list
type nodeL []storageproto.Node
func (nl nodeL) Len() int { return len(nl) }
func (nl nodeL) Swap(i, j int) { nl[i], nl[j] = nl[j], nl[i]}

type byID struct{ nodeL }
func (c byID) Less(i, j int) bool { return c.nodeL[i].NodeID < c.nodeL[j].NodeID }



func iNewStorageserver(buddy string, portnum int, nodeid uint32) *Storageserver {

	// //fmt.Println("called new storage server")

	ss := &Storageserver{}

	//if no nodeid is provided, choose one randomly
	if nodeid == 0 {
		reallySeedTheDamnRNG()
		ss.nodeid = rand.Uint32()
	} else {
		//otherwise just take the one they gave you
		ss.nodeid = nodeid
	}

	ss.portnum = portnum

	ss.nodeList = []storageproto.Node{}
	ss.nodeListM = make(chan int, 1)
	ss.nodeListM <- 1

	ss.connMap = make(map[int]*rpc.Client)
	ss.connMapM = make(chan int, 1)
	ss.connMapM <- 1

	ss.valMap = make(map[string]string)
	ss.valMapM = make(chan int, 1)
	ss.valMapM <- 1

	/*right now we are assuming that the buddy node won't fail. need to change for later*/
	for err != nil {
		//keep retrying until we can actually conenct
		//(buddy may not have started yet)
		buddyNode, err = rpc.DialHTTP("tcp", buddy)
	// 	//fmt.Println("Trying to connect to buddy...")
		time.Sleep(time.Duration(3) * time.Second)
	}

	/*gotta send rpc call to buddy to get ourselves on everyone's list of nodes 
	and also get the list of nodes for ourselves. */

	//set up args for registering ourselves
	info := storageproto.Node{HostPort: "localhost:" + strconv.Itoa(portnum), NodeID: ss.nodeid}
	args := storageproto.RegisterArgs{ServerInfo: info}
	reply := storageproto.RegisterReply{}

	for err != nil || reply.Ready != true {
		//call register on the master node with our info as the args. Kinda weird
		err = buddyNode.Call("StorageRPC.Register", &args, &reply)
		//keep retrying until all things are registered
		//fmt.Println("Trying to register with master...")
		time.Sleep(time.Duration(3) * time.Second)
	}

	//now gotta get the reply and do stuff with it
	<- ss.nodeListM
	ss.nodeList = reply.Servers
	ss.nodeListM <- 1

	log.Println("Successfully joined storage node cluster.")
	/*slist := ""
	for i := 0; i < len(ss.nodeList); i++ {
		res := fmt.Sprintf("{localhost:%v %v}", ss.portnum, ss.nodeid)
		slist += res
		if i < len(ss.nodeList)-1 {
		slist += " "
		}
	}
	log.Printf("Server List: [%s]", slist)*/

	//now we should tell all other servers about our existance....
	for index, hostport := range ss.nodeList {
		//again we are assuming that none of the node die...
		for err != nil {
			//keep retrying until we can actually conenct
			//(buddy may not have started yet)
			servNode, err = rpc.DialHTTP("tcp", hostport)
		// 	//fmt.Println("Trying to connect to buddy...")
			time.Sleep(time.Duration(3) * time.Second)
		}
		for err != nil || reply.Ready != true {
			//call register on the master node with our info as the args. Kinda weird
			err = servNode.Call("StorageRPC.Register", &args, &reply)
			//keep retrying until all things are registered
			//fmt.Println("Trying to register with master...")
			time.Sleep(time.Duration(3) * time.Second)
		}
		//make sure to close the connection!
		err := serveNode.Close()
	}

	//now that we have registered with all other nodes and our list of servers is up to date, we 
	//want to find out which are going to be in our skiplist

	numNodes := len(ss.nodeList)

	jump := numNodes / 4

	if numNodes > 1 {
		if numNodes <= 5 {
			//if we have less than five nodes just connect to every other node
			for index, node := range ss.nodeList {
				/*right now we are assuming that the buddy node won't fail. need to change for later*/
				for err != nil {
					//keep retrying until we can actually conenct
					//(buddy may not have started yet)
					buddyNode, err := rpc.DialHTTP("tcp", node.HostPort)
				// 	//fmt.Println("Trying to connect to buddy...")
					time.Sleep(time.Duration(3) * time.Second)
				}
				<- ss.connMapM
				ss.connMap[node.NodeID] = buddyNode
				ss.connMap <- 1
			}
		} else {
			//otherwise it's math time!
			var buddyList []storageproto.Node
			for index, hostport := range ss.nodeList {
				if hostport == ("localhost:" + strconv.Itoa(portnum)) {
					buddyList = append(buddyList, ss.nodeList[(index-1)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+1)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+jump)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+2*jump)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+3*jump)%numNodes])
				}
			}
			for index, node := range buddyList {
				/*right now we are assuming that the buddy node won't fail. need to change for later*/
				for err != nil {
					//keep retrying until we can actually conenct
					//(buddy may not have started yet)
					buddyNode, err := rpc.DialHTTP("tcp", node.HostPort)
				// 	//fmt.Println("Trying to connect to buddy...")
					time.Sleep(time.Duration(3) * time.Second)
				}
				<- ss.connMapM
				ss.connMap[node.NodeID] = buddyNode
				ss.connMap <- 1
			}
		}
	}

	ss.srpc = storagerpc.NewStorageRPC(ss)
	rpc.Register(ss.srpc)
	//go ss.GarbageCollector()

	// //fmt.Println("started new server")
	// /*fmt.Println(storageproto.Node{HostPort: "localhost:" + strconv.Itoa(portnum), NodeID: ss.nodeid})
	// fmt.Printf("master? %v\n", ss.isMaster)
	// fmt.Printf("numnodes? %v\n", ss.numNodes)*/

	return ss
}

// called by a new server on all other servers when it joins
func (ss *Storageserver) RegisterServer(args *storageproto.RegisterArgs, reply *storageproto.RegisterReply) error {

	//fmt.Println("called register server")

	<- ss.nodeListM
	//fmt.Println("aquired nodeMap lock RegisterServer")
	ok := false
	for index, node := range ss.nodeList {
		if (node == args.ServerInfo) {
			ok = true
			break
		}
	}
	ss.nodeListM <- 1
	//if not we have to add it to the map and to the list
	if ok != true {
		//put it in the list
		<-ss.nodeListM
		//fmt.Println("aquired nodeList lock RegisterServer")
		ss.nodeList = append(ss.nodeList, args.ServerInfo)
		ss.nodeListM <- 1
		//fmt.Println("release nodeList lock RegisterServer")
	}

	sort.Sort(byID{ss.nodelist})

	//send back the list of servers
	reply.Servers = ss.nodeList

	//redo skip list

	<- ss.connMapM 
		for key, value := range ss.connMap {
			delete(ss.connMap, key)
		}
	ss.connMapM <- 1

	numNodes := len(ss.nodeList)

	jump := numNodes / 4

	if numNodes > 1 {
		if numNodes <= 5 {
			//if we have less than five nodes just connect to every other node
			for index, node := range ss.nodeList {
				/*right now we are assuming that the buddy node won't fail. need to change for later*/
				for err != nil {
					//keep retrying until we can actually conenct
					//(buddy may not have started yet)
					buddyNode, err := rpc.DialHTTP("tcp", node.HostPort)
				// 	//fmt.Println("Trying to connect to buddy...")
					time.Sleep(time.Duration(3) * time.Second)
				}
				<- ss.connMapM
				ss.connMap[node.NodeID] = buddyNode
				ss.connMap <- 1
			}
		} else {
			//otherwise it's math time!
			var buddyList []storageproto.Node
			for index, hostport := range ss.nodeList {
				if hostport == ("localhost:" + strconv.Itoa(portnum)) {
					buddyList = append(buddyList, ss.nodeList[(index-1)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+1)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+jump)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+2*jump)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+3*jump)%numNodes])
				}
			}
			for index, node := range buddyList {
				/*right now we are assuming that the buddy node won't fail. need to change for later*/
				for err != nil {
					//keep retrying until we can actually conenct
					//(buddy may not have started yet)
					buddyNode, err := rpc.DialHTTP("tcp", node.HostPort)
				// 	//fmt.Println("Trying to connect to buddy...")
					time.Sleep(time.Duration(3) * time.Second)
				}
				<- ss.connMapM
				ss.connMap[node.NodeID] = buddyNode
				ss.connMap <- 1
			}
		}
	}

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
