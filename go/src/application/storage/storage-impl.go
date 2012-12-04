/*LIZ TODO:
-check for node failure
-deal with node failure
	-route to differnt people
	-update nodeList and connMap(skiplist)
	-delt with on the userclient side:
		-recover lost files
		-need to check with midclients who connect if we have all their files
		-if not, put those files
-keep track of people who have acess to a file so we can propogate
	-how the fuck do we even do this what
		-push file json to midclient, midclient pushes to userclient
-check that permissions stuff works
	-every time recieve a put, check permissions and update list
		-only person who owns file can change permissions
	-when recieving a put, check if we already have file
		-if yes can overwrite if person has overwrite permissions
		-if they don't return some error
	-when fulfilling a get, check permissions to see if person has acess to file
-keep track of connections to midclients and what users they are associated with
-auto push changes to files to relevant users
*/

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
	"application/storagerpc"
	crand "crypto/rand"
	"fmt"
	"hash/fnv"
	"log"
	"math"
	"math/big"
	"math/rand"
	"net"
	"net/rpc"
	// "packages/lsplog"
	"protos/storageproto"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Storageserver struct {
	nodeid  uint32
	portnum int

	nodeIndex int                 // index in nodeList
	nodeList  []storageproto.Node //list of all other nodes and portnumbers, SORTED
	nodeListM chan int

	connMap  map[uint32]*rpc.Client //map from nodeID to connection for your skiplist
	connMapM chan int

	valMap  map[string]string //map of actual stuff we are storing... I think
	valMapM chan int

	srpc *storagerpc.StorageRPC
}

func reallySeedTheDamnRNG() {
	randint, _ := crand.Int(crand.Reader, big.NewInt(math.MaxInt64))
	rand.Seed(randint.Int64())
}

//set up sorting for the storage server list
type nodeL []storageproto.Node

func (nl nodeL) Len() int      { return len(nl) }
func (nl nodeL) Swap(i, j int) { nl[i], nl[j] = nl[j], nl[i] }

type byID struct{ nodeL }

func (c byID) Less(i, j int) bool { return c.nodeL[i].NodeID < c.nodeL[j].NodeID }

func iNewStorageserver(buddy string, portnum int, nodeid uint32) *Storageserver {
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

	ss.nodeList = append([]storageproto.Node{}, storageproto.Node{net.JoinHostPort("localhost", strconv.Itoa(portnum)), ss.nodeid})
	ss.nodeListM = make(chan int, 1)
	ss.nodeListM <- 1

	ss.connMap = make(map[uint32]*rpc.Client)
	ss.connMapM = make(chan int, 1)
	ss.connMapM <- 1

	ss.valMap = make(map[string]string)
	ss.valMapM = make(chan int, 1)
	ss.valMapM <- 1

	if buddy == "" {
		ss.srpc = storagerpc.NewStorageRPC(ss)
		rpc.Register(ss.srpc)
		fmt.Println("Storage server started. My NodeID is", ss.nodeid)
		return ss
	}

	/*right now we are assuming that the buddy node won't fail. need to change for later*/
	buddyNode, err := rpc.DialHTTP("tcp", buddy)
	for err != nil {
		//keep retrying until we can actually conenct
		//(buddy may not have started yet)
		buddyNode, err = rpc.DialHTTP("tcp", buddy)
		time.Sleep(time.Duration(3) * time.Second)
	}

	/*gotta send rpc call to buddy to get ourselves on everyone's list of nodes 
	and also get the list of nodes for ourselves. */

	//set up args for registering ourselves
	info := storageproto.Node{HostPort: "localhost:" + strconv.Itoa(portnum), NodeID: ss.nodeid}
	args := storageproto.RegisterArgs{ServerInfo: info}
	reply := storageproto.RegisterReply{}

	fmt.Println("Registering in New")
	err = buddyNode.Call("StorageRPC.Register", &args, &reply)
	fmt.Println("Finished dialing in New")
	for err != nil {
		//call register on the master node with our info as the args. Kinda weird
		err = buddyNode.Call("StorageRPC.Register", &args, &reply)
		//keep retrying until all things are registered
		// fmt.Println("Trying to register with master...")
		time.Sleep(time.Duration(3) * time.Second)
	}

	//now gotta get the reply and do stuff with it
	<-ss.nodeListM
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
	for _, node := range ss.nodeList {
		//again we are assuming that none of the node die...
		if node.NodeID != ss.nodeid {
			servNode, err := rpc.DialHTTP("tcp", node.HostPort)
			for err != nil {
				//keep retrying until we can actually connect
				//(buddy may not have started yet)
				servNode, err = rpc.DialHTTP("tcp", node.HostPort)
				// 	//fmt.Println("Trying to connect to buddy...")
				time.Sleep(time.Duration(3) * time.Second)
			}
			err = servNode.Call("StorageRPC.Register", &args, &reply)
			for err != nil {
				//call register on the master node with our info as the args. Kinda weird
				err = servNode.Call("StorageRPC.Register", &args, &reply)
				//keep retrying until all things are registered
				//fmt.Println("Trying to register with master...")
				time.Sleep(time.Duration(3) * time.Second)
			}
			//make sure to close the connection!
			err = servNode.Close()
		}
	}

	//now that we have registered with all other nodes and our list of servers is up to date, we 
	//want to find out which are going to be in our skiplist

	numNodes := len(ss.nodeList)

	jump := numNodes / 4

	if numNodes > 1 {
		if numNodes <= 5 {
			//if we have less than five nodes just connect to every other node
			for _, node := range ss.nodeList {
				if node.NodeID != ss.nodeid {
					/*right now we are assuming that the buddy node won't fail. need to change for later*/
					buddyNode, err := rpc.DialHTTP("tcp", node.HostPort)
					for err != nil {
						//keep retrying until we can actually conenct
						//(buddy may not have started yet)
						buddyNode, err = rpc.DialHTTP("tcp", node.HostPort)
						// 	//fmt.Println("Trying to connect to buddy...")
						time.Sleep(time.Duration(3) * time.Second)
					}
					<-ss.connMapM
					ss.connMap[node.NodeID] = buddyNode
					ss.connMapM <- 1
				}
			}
		} else {
			//otherwise it's math time!
			var buddyList []storageproto.Node
			for index, node := range ss.nodeList {
				if node.NodeID != ss.nodeid && node.HostPort == ("localhost:"+strconv.Itoa(portnum)) {
					buddyList = append(buddyList, ss.nodeList[(index-1)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+1)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+jump)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+2*jump)%numNodes])
					buddyList = append(buddyList, ss.nodeList[(index+3*jump)%numNodes])
				}
			}
			for _, node := range buddyList {
				/*right now we are assuming that the buddy node won't fail. need to change for later*/
				buddyNode, err := rpc.DialHTTP("tcp", node.HostPort)
				if node.NodeID != ss.nodeid {
					for err != nil {
						//keep retrying until we can actually conenct
						//(buddy may not have started yet)
						buddyNode, err = rpc.DialHTTP("tcp", node.HostPort)
						// 	//fmt.Println("Trying to connect to buddy...")
						time.Sleep(time.Duration(3) * time.Second)
					}
					<-ss.connMapM
					ss.connMap[node.NodeID] = buddyNode
					ss.connMapM <- 1
				}
			}
		}
	}
	<-ss.nodeListM
	sort.Sort(byID{ss.nodeList})
	for i := 0; i < len(ss.nodeList); i++ {
		if ss.nodeList[i].NodeID == ss.nodeid {
			ss.nodeIndex = i
			break
		}
	}
	ss.nodeListM <- 1

	ss.srpc = storagerpc.NewStorageRPC(ss)
	rpc.Register(ss.srpc)
	fmt.Println("Storage server started. My NodeID is", ss.nodeid)
	fmt.Printf("I am aware of the following nodes: %v\n", ss.nodeList)
	go ss.iCalculateSkipList()

	//go ss.GarbageCollector()

	return ss
}

func (ss *Storageserver) iCalculateSkipList() {
	<-ss.connMapM
	for key, _ := range ss.connMap {
		ss.connMap[key].Close()
		delete(ss.connMap, key)
	}
	ss.connMapM <- 1

	numNodes := len(ss.nodeList)

	jump := numNodes / 4

	if numNodes <= 6 {
		//if we have less than five nodes just connect to every other node
		// fmt.Println("nodes less than 5!")
		for _, node := range ss.nodeList {
			if node.NodeID != ss.nodeid {
				/*right now we are assuming that the buddy node won't fail. need to change for later*/
				// fmt.Printf("Dialing %v from %v...\n", node.HostPort, ss.portnum)
				buddyNode, err := rpc.DialHTTP("tcp", node.HostPort)
				// fmt.Println("Successful dial!...\n")
				for err != nil {
					//keep retrying until we can actually conenct
					//(buddy may not have started yet)
					buddyNode, err = rpc.DialHTTP("tcp", node.HostPort)
					time.Sleep(time.Duration(3) * time.Second)
				}
				<-ss.connMapM
				ss.connMap[node.NodeID] = buddyNode
				ss.connMapM <- 1
			}
		}
	} else {
		fmt.Println("making a buddyList by math since there are more than six nodes")
		//otherwise it's math time!
		var buddyList []storageproto.Node
		index := ss.nodeIndex
		fmt.Println("index, index+1, index-1, index+jump, index+2*jump, index+3*jump all mod numNodes")
		fmt.Println(index, (index-1)%numNodes, (index+1)%numNodes, (index+jump)%numNodes, (index+2*jump)%numNodes, (index+3*jump)%numNodes)
		first := index - 1
		if first < 0 {
			first = -first
		}
		buddyList = append(buddyList, ss.nodeList[(first)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+1)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+jump)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+2*jump)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+3*jump)%numNodes])
		fmt.Println("figued out who is in buddyList")

		fmt.Println("Buddy node won't fail. Check for other shit")
		for _, node := range buddyList {
			if node.NodeID != ss.nodeid {
				/*right now we are assuming that the buddy node won't fail. need to change for later*/
				buddyNode, err := rpc.DialHTTP("tcp", node.HostPort)
				for err != nil {
					//keep retrying until we can actually conenct
					//(buddy may not have started yet)
					buddyNode, err = rpc.DialHTTP("tcp", node.HostPort)
					// 	//fmt.Println("Trying to connect to buddy...")
					time.Sleep(time.Duration(3) * time.Second)
				}
				<-ss.connMapM
				ss.connMap[node.NodeID] = buddyNode
				ss.connMapM <- 1
			}
		}
	}
	fmt.Printf("I am aware of the following nodes: %v\n", ss.nodeList)
	fmt.Printf("What I think my buddy list is %v\n", ss.connMap)
}

// called by a new server on all other servers when it joins
func (ss *Storageserver) RegisterServer(args *storageproto.RegisterArgs, reply *storageproto.RegisterReply) error {
	<-ss.nodeListM
	// fmt.Println("aquired nodeMap lock RegisterServer")
	ok := false
	for _, node := range ss.nodeList {
		if node == args.ServerInfo {
			ok = true
			break
		}
	}
	ss.nodeListM <- 1
	//if not we have to add it to the map and to the list
	if ok != true {
		//put it in the list
		<-ss.nodeListM
		// fmt.Println("aquired nodeList lock RegisterServer")
		ss.nodeList = append(ss.nodeList, args.ServerInfo)
		ss.nodeListM <- 1
		// fmt.Println("release nodeList lock RegisterServer")
	}

	<-ss.nodeListM
	sort.Sort(byID{ss.nodeList})
	for i := 0; i < len(ss.nodeList); i++ {
		if ss.nodeList[i].NodeID == ss.nodeid {
			ss.nodeIndex = i
			break
		}
	}
	ss.nodeListM <- 1

	//send back the list of servers
	reply.Servers = ss.nodeList

	//redo skip list
	go ss.iCalculateSkipList()
	return nil
}

func Storehash(key string) uint32 {
	hasher := fnv.New32()
	hasher.Write([]byte(key))
	return hasher.Sum32()
}

func (ss *Storageserver) checkServer(key string) (*rpc.Client, bool) {
	userclass := strings.Split(key, "?")[0]
	keyid := Storehash(userclass)

	fmt.Printf("\nStorehash: %v\nServehash: %v\n\n", keyid, ss.nodeid)
	fmt.Printf("What I think the node array is %v\n", ss.nodeList)
	fmt.Printf("What I think my buddy list is %v\n", ss.connMap)
	fmt.Printf("Node index: %v\n", ss.nodeIndex)

	var predecessor int
	if ss.nodeIndex == 0 {
		predecessor = len(ss.nodeList) - 1
	} else {
		predecessor = ss.nodeIndex - 1
	}

	fmt.Printf("Predecessor index:%v\n", predecessor)

	if (keyid < ss.nodeList[ss.nodeIndex].NodeID && ss.nodeIndex == 0) ||
		(keyid > ss.nodeList[len(ss.nodeList)-1].NodeID && ss.nodeIndex == 0) ||
		(keyid > ss.nodeList[predecessor].NodeID && keyid <= ss.nodeList[ss.nodeIndex].NodeID) {
		fmt.Println("This is the correct server")
		return nil, true
	}

	fmt.Println("OHFUCK Either the wrong server or this number is at the end of the circle.")

	for nodeId, nodeClient := range ss.connMap {
		fmt.Println("finding another node to give it to. Checking: ", nodeId)
		if keyid < nodeId {
			for nodeId2, _ := range ss.connMap {
				if nodeId > nodeId2 && keyid < nodeId2 {
					nodeId = nodeId2
				}
			}
			fmt.Println("nodeID of node I'm passing it to is: ", nodeId)
			return nodeClient, false
		}
	}

	//if we get down here, this means that the key hit wraparound, so we should send it to the lowest nodeId in the skiplist
	lowestNodeId := ss.nodeList[len(ss.nodeList)-1].NodeID
	for nodeId, _ := range ss.connMap {
		if nodeId < lowestNodeId {
			lowestNodeId = nodeId
		}
	}
	for nodeId, nodeClient := range ss.connMap {
		if nodeId == lowestNodeId {
			return nodeClient, false
		}
	}

	fmt.Println("couldn't find another guy to give it to!")
	//Uh, should never get here hopefully
	return nil, true
}

// RPC-able interfaces, bridged via StorageRPC.
// These should do something! :-)

func (ss *Storageserver) iGet(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	fmt.Println("Client called Get")
	server, correct := ss.checkServer(args.Key)

	if correct == false {
		err := server.Call("StorageRPC.Get", args, reply)
		if err != nil {
			return err
		}
		return nil
	}

	<-ss.valMapM
	val, ok := ss.valMap[args.Key]
	if ok != true {
		reply.Status = storageproto.EKEYNOTFOUND
		reply.JSONFile = ""
	} else {
		reply.Status = storageproto.OK
		reply.JSONFile = val
	}
	ss.valMapM <- 1

	return nil
}

func (ss *Storageserver) iPut(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	fmt.Println("Client called Put")
	server, correct := ss.checkServer(args.Key)

	if correct == false {
		fmt.Println("Oh fuck.")
		err := server.Call("StorageRPC.Put", args, reply)
		if err != nil {
			return err
		}
		return nil
	}

	<-ss.valMapM
	ss.valMap[args.Key] = args.JSONFile
	ss.valMapM <- 1

	reply.Status = storageproto.OK
	return nil
}

func (ss *Storageserver) iDelete(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	fmt.Println("Client called Delete")
	server, correct := ss.checkServer(args.Key)

	if correct == false {
		err := server.Call("StorageRPC.Delete", args, reply)
		if err != nil {
			return err
		}
		return nil
	}

	<-ss.valMapM
	delete(ss.valMap, args.Key)
	ss.valMapM <- 1

	reply.Status = storageproto.OK
	return nil
}
