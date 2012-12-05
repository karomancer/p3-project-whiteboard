/*LIZ TODO:
√check for node failure
√deal with node failure
	-route to differnt people
	-update nodeList and connMap(skiplist)
	-delt with on the userclient side:
		-recover lost files
		-need to check with midclients who connect if we have all their files
		-if not, put those files
-keep track of people who have acess to a file so we can propogate
	-how the fuck do we even do this what
		-push file json to midclient, midclient pushes to userclient
√check that permissions stuff works
	√every time recieve a put, check permissions and update list (this is in usrclient)
		√only person who owns file can change permissions
	√when recieving a put, check if we already have file
		√if yes can overwrite if person has overwrite permissions
		√if they don't return some error
	√when fulfilling a get, check permissions to see if person has acess to file
-keep track of connections to midclients and what users they are associated with
-auto push changes to files to relevant users

√when new server joins:
√look at node right after it, if that node isn't dead, get data that now hashes to you rather than to it
√delete that data from the node

*/

package storage

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
	"encoding/json"
)

type Storageserver struct {
	nodeid  uint32
	portnum int

	nodeIndex int                 // index in nodeList
	nodeList  []storageproto.Node //list of all other nodes and portnumbers, SORTED
	nodeListM chan int

	deadNodeList []storageproto.Node
	deadNodeListM chan int

	connMap  map[uint32]*rpc.Client //map from nodeID to connection for your skiplist
	connMapM chan int

	fileMap  map[string]storageproto.SyncFile
	fileMapM chan int

	userMap map[string]string
	userMapM chan int

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

	ss.deadNodeList = []storageproto.Node{}
	ss.deadNodeListM = make(chan int, 1)
	ss.deadNodeListM <- 1

	ss.connMap = make(map[uint32]*rpc.Client)
	ss.connMapM = make(chan int, 1)
	ss.connMapM <- 1

	ss.fileMap = make(map[string]storageproto.SyncFile)
	ss.fileMapM = make(chan int, 1)
	ss.fileMapM <- 1

	ss.userMap = make(map[string]string)
	ss.userMapM = make(chan int, 1)
	ss.userMapM <- 1

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

	/*registering with buddy node, who we are assuming doesn't die at this point....*/
	//fmt.Println("Registering in New")
	err = buddyNode.Call("StorageRPC.Register", &args, &reply)
	//fmt.Println("Finished dialing in New")
	for err != nil {
		//call register on the master node with our info as the args. Kinda weird
		err = buddyNode.Call("StorageRPC.Register", &args, &reply)
		//keep retrying until all things are registered
		// //fmt.Println("Trying to register with master...")
		time.Sleep(time.Duration(3) * time.Second)
	}

	//now gotta get the reply and do stuff with it
	<-ss.nodeListM
	ss.nodeList = reply.Servers
	ss.nodeListM <- 1

	log.Println("Successfully joined storage node cluster.")
	//fmt.Println("NodeList: ", ss.nodeList)

	//now we should tell all other servers about our existance....
	for _, node := range ss.nodeList {
		//again we are assuming that none of the node die...
		if node.NodeID != ss.nodeid {
			//fmt.Println("current node: ", node.NodeID)
			nodeConnection, success := ss.dialNode(node)
			if success == true {
				//fmt.Println("able to connect to node!")
				success = ss.registerWithNode(node, nodeConnection, args, reply)
				//make sure to close the connection!
				if success == true {
					//fmt.Println("going to close connection")
					err = nodeConnection.Close()
				}
			}
			//fmt.Println("done with node: ", node.NodeID)
		}
	}

	//fmt.Println("about to collectBodies in new")

	ss.collectBodies()

	<-ss.nodeListM
	sort.Sort(byID{ss.nodeList})
	for i := 0; i < len(ss.nodeList); i++ {
		if ss.nodeList[i].NodeID == ss.nodeid {
			ss.nodeIndex = i
			break
		}
	}
	ss.nodeListM <- 1

	fromNodeIndex := 0
	if ss.nodeIndex == len(ss.nodeList) - 1 {
		fromNodeIndex = 0
	} else {
		fromNodeIndex = ss.nodeIndex + 1
	}

	//open connection 
	nodeConn, success := ss.dialNode(ss.nodeList[fromNodeIndex])

	if success == true {
		toNode := storageproto.Node{HostPort: "localhost:" + strconv.Itoa(portnum), NodeID: ss.nodeid}
		//set up put args
		transferArgs := &storageproto.TransferArgs{toNode}
		//set up put reply
		var transferReply storageproto.TransferReply
		//actually put
		//fmt.Println("transfering Data")
		nodeConn.Call("StorageRPC.Transfer", transferArgs, &transferReply)
		//fmt.Println("done getting data, now adding to our stores")

		<- ss.userMapM
		for key, val := range transferReply.UserMap {
			ss.userMap[key] = val
		}
		ss.userMapM <- 1

		<- ss.fileMapM
		for key, val := range transferReply.FileMap {
			var file storageproto.SyncFile
			fileBytes := []byte(val)
			json.Unmarshal(fileBytes, &file)

			ss.fileMap[key] = file

		}
		ss.fileMapM <- 1

	}

	//fmt.Println("done transfering data")

	ss.srpc = storagerpc.NewStorageRPC(ss)
	rpc.Register(ss.srpc)
	fmt.Println("Storage server started. My NodeID is", ss.nodeid)
	fmt.Printf("I am aware of the following nodes: %v\n", ss.nodeList)
	////fmt.Println("This is my buddyList: ", ss.connMap)

	go ss.iCalculateSkipList()

	return ss
}

//function to transfer data from one node to another because of reasons (consistant hashing meaning that data may change nodes if more nodes are added)
func (ss *Storageserver) TransferData(args *storageproto.TransferArgs, reply *storageproto.TransferReply) error {

	//fmt.Println("called transfer data")

	if (len(ss.userMap) == 0) && (len(ss.fileMap) == 0) {
		//fmt.Println("no data to transfer")
		return nil
	}

	toNodeID := args.ToNode.NodeID

	//need to get nodeIndex for the node we are sending to
	toNodeIndex := 0
	<-ss.nodeListM
	sort.Sort(byID{ss.nodeList})
	for i := 0; i < len(ss.nodeList); i++ {
		if ss.nodeList[i].NodeID == ss.nodeid {
			toNodeIndex = i
			break
		}
	}
	ss.nodeListM <- 1

	<- ss.fileMapM
	//fmt.Println("starting to transfer files")
	reply.FileMap = make(map[string]string)
	for key, file := range ss.fileMap {
		userclass := strings.Split(key, "?")[0]
		keyid := Storehash(userclass)
		if (keyid <= toNodeID) ||
		(keyid > ss.nodeList[len(ss.nodeList)-1].NodeID && toNodeIndex == 0) {
			fileJSON, _ := json.Marshal(file)
			reply.FileMap[key] = string(fileJSON)
			delete(ss.fileMap, key)
		}
	}
	ss.fileMapM <- 1
	//fmt.Println("done transfering files")

	//need to do the same stuff but for users
	<- ss.userMapM
	//fmt.Println("starting to transfer user data")
	reply.UserMap = make(map[string]string)
	for key, data := range ss.userMap {
		userclass := strings.Split(key, "?")[0]
		keyid := Storehash(userclass)
		if (keyid <= toNodeID) ||
		(keyid > ss.nodeList[len(ss.nodeList)-1].NodeID && toNodeIndex == 0) {
			reply.UserMap[key] = data
			delete(ss.userMap, key)
		}
	}
	ss.userMapM <- 1
	//fmt.Println("done transfering user data")

	//fmt.Println("everything is hunky dory")

	reply.Status = storageproto.OK
	return nil
} 

//function to clean up dead nodes
func (ss *Storageserver) collectBodies() {

	//fmt.Println("called CollectBodies") 

	<- ss.deadNodeListM

	if len(ss.deadNodeList) == 0 {
		ss.deadNodeListM <- 1
		return
	}

	for _, deadNode := range ss.deadNodeList {
		<- ss.nodeListM
		for index, node := range ss.nodeList {
			if deadNode.NodeID == node.NodeID {
				if index < len(ss.nodeList) - 1 {
					ss.nodeList = append(ss.nodeList[:index], ss.nodeList[index + 1])
				} else {
					ss.nodeList = ss.nodeList[:index]
				}
			}
		}
		ss.nodeListM <- 1
	}
	ss.deadNodeList = []storageproto.Node{}
	ss.deadNodeListM <- 1

	//fmt.Println("nodeList after collecting dead nodes: ", ss.nodeList)

	<-ss.nodeListM
	sort.Sort(byID{ss.nodeList})
	for i := 0; i < len(ss.nodeList); i++ {
		if ss.nodeList[i].NodeID == ss.nodeid {
			ss.nodeIndex = i
			break
		}
	}
	ss.nodeListM <- 1

	ss.iCalculateSkipList()
}

//function to make connections with nodes and check for failure
func (ss *Storageserver) dialNode(node storageproto.Node) (*rpc.Client, bool) {
	//fmt.Println("dialNode")
	//fmt.Println("dialing node: with id:", node.HostPort, node.NodeID)
	nodeClient, err := rpc.DialHTTP("tcp", node.HostPort)
	//fmt.Println("dialed once")
	if err != nil {
		//fmt.Println("dialing once didn't work, trying 5 times with 3 second wait")
		tries := 5
		for tries > 0 {
			//keep retrying until we can actually conenct
			//(buddy may not have started yet)
			nodeClient, err = rpc.DialHTTP("tcp", node.HostPort)
			if err == nil {
				break
			}
			time.Sleep(time.Duration(3) * time.Second)
			tries --
		}
	}

	//fmt.Println("done tyring to connect")

	if err != nil {
		//fmt.Println("could not connect, adding node to deadList")
		<- ss.deadNodeListM
		//fmt.Println("successfully locked deadlist")
		ss.deadNodeList = append(ss.deadNodeList, node)
		//fmt.Println("added node to deadlist")
		ss.deadNodeListM <- 1
		//fmt.Println("unlocked deadlist mutex")

		return nil, false
	}

	//fmt.Println("able to connect!")

	return nodeClient, true
}

//function to register with other nodes and check to make sure they aren't dead
func (ss *Storageserver) registerWithNode(node storageproto.Node, servNode *rpc.Client, args storageproto.RegisterArgs, reply storageproto.RegisterReply) bool {
	err := servNode.Call("StorageRPC.Register", &args, &reply)

	if err != nil {
		tries := 5
		for tries > 0 {
			//call register on the master node with our info as the args. Kinda weird
			err = servNode.Call("StorageRPC.Register", &args, &reply)
			if err == nil {
				break
			}
			//keep retrying until all things are registered
			////fmt.Println("Trying to register with master...")
			time.Sleep(time.Duration(3) * time.Second)
			tries --
		}
	}

	if err != nil {
		<- ss.deadNodeListM
		ss.deadNodeList = append(ss.deadNodeList, node)
		ss.deadNodeListM <- 1

		return false
	}

	return true
}

func (ss *Storageserver) iCalculateSkipList() {
	<-ss.connMapM
	for key, _ := range ss.connMap {
		//fmt.Println("map endtry at key: ", ss.connMap[key])
		if ss.connMap[key] != nil {
			ss.connMap[key].Close()
		}	
		delete(ss.connMap, key)
	}
	ss.connMapM <- 1

	numNodes := len(ss.nodeList)

	jump := numNodes / 4

	if numNodes <= 6 {
		//if we have less than five nodes just connect to every other node
		// //fmt.Println("nodes less than 5!")
		for _, node := range ss.nodeList {
			if node.NodeID != ss.nodeid {
				buddyNode, _ := ss.dialNode(node)
				<-ss.connMapM
				ss.connMap[node.NodeID] = buddyNode
				ss.connMapM <- 1
			}
		}
		go ss.collectBodies()
	} else {
		//fmt.Println("making a buddyList by math since there are more than six nodes")
		//otherwise it's math time!
		var buddyList []storageproto.Node
		index := ss.nodeIndex
		first := index - 1
		if first < 0 {
			first = -first
		}
		buddyList = append(buddyList, ss.nodeList[(first)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+1)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+jump)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+2*jump)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+3*jump)%numNodes])
		//fmt.Println("figured out who is in buddyList")

		for _, node := range buddyList {
			if node.NodeID != ss.nodeid {
				buddyNode, success := ss.dialNode(node)
				if success == true {
					<-ss.connMapM
					ss.connMap[node.NodeID] = buddyNode
					ss.connMapM <- 1
				}
			}
		}
		go ss.collectBodies()
	}
	//fmt.Printf("I am aware of the following nodes: %v\n", ss.nodeList)
	//fmt.Printf("What I think my buddy list is %v\n", ss.connMap)
}

// called by a new server on all other servers when it joins
func (ss *Storageserver) RegisterServer(args *storageproto.RegisterArgs, reply *storageproto.RegisterReply) error {
	<-ss.nodeListM
	// //fmt.Println("aquired nodeMap lock RegisterServer")
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
		// //fmt.Println("aquired nodeList lock RegisterServer")
		ss.nodeList = append(ss.nodeList, args.ServerInfo)
		ss.nodeListM <- 1
		// //fmt.Println("release nodeList lock RegisterServer")
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

func (ss *Storageserver) checkServer(key string) (storageproto.Node, *rpc.Client, bool) {

	fmt.Println("called check server")

	userclass := strings.Split(key, "?")[0]
	keyid := Storehash(userclass)

	<-ss.nodeListM
	fmt.Println("locked nodeLIst")
	fmt.Println("checking my nodeIndex")
	fmt.Println("what I think node list looks like: ", ss.nodeList)
	sort.Sort(byID{ss.nodeList})
	fmt.Println("sorted nodeList")
	for i := 0; i < len(ss.nodeList); i++ {
		fmt.Println("looking for my new nodeIndex")
		if ss.nodeList[i].NodeID == ss.nodeid {
			fmt.Println("I found my new index! It's: ", i)
			ss.nodeIndex = i
			break
		}
	}
	fmt.Println("don't need a new nodeIndex")
	if len(ss.nodeList) < ss.nodeIndex {
		//pretty hacky. Basically if something goes horribly wrogn with too many good servers dying we designate ourselves as the correct server
		//this is PROVISIONAL, and this case barely ever happens...
		return storageproto.Node{}, nil, true
	}
	ss.nodeListM <- 1
	fmt.Println("unlocked nodelist")


	//fmt.Printf("\nStorehash: %v\nServehash: %v\n\n", keyid, ss.nodeid)
	//fmt.Printf("What I think the node list is %v\n", ss.nodeList)
	//fmt.Printf("What I think my buddy list is %v\n", ss.connMap)
	//fmt.Printf("Node index: %v\n", ss.nodeIndex)

	var predecessor int
	if ss.nodeIndex == 0 {
		predecessor = len(ss.nodeList) - 1
	} else {
		predecessor = ss.nodeIndex - 1
	}

	//fmt.Printf("Predecessor index:%v\n", predecessor)

	if (keyid <= ss.nodeList[ss.nodeIndex].NodeID && ss.nodeIndex == 0) ||
		(keyid > ss.nodeList[len(ss.nodeList)-1].NodeID && ss.nodeIndex == 0) ||
		(keyid > ss.nodeList[predecessor].NodeID && keyid <= ss.nodeList[ss.nodeIndex].NodeID) {
		fmt.Println("This is the correct server")
		return storageproto.Node{}, nil, true
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

			nodeToSendTo := storageproto.Node{}

			<- ss.nodeListM
			for _, node := range ss.nodeList {
				if node.NodeID == nodeId {
					nodeToSendTo = node
				}
			}
			ss.nodeListM <- 1

			return nodeToSendTo, nodeClient, false
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

			nodeToSendTo := storageproto.Node{}

			<- ss.nodeListM
			for _, node := range ss.nodeList {
				if node.NodeID == nodeId {
					nodeToSendTo = node
				}
			}
			ss.nodeListM <- 1

			return nodeToSendTo, nodeClient, false
		}
	}

	fmt.Println("couldn't find another guy to give it to!")
	//Uh, should never get here hopefully
	return storageproto.Node{}, nil, true
}

// RPC-able interfaces, bridged via StorageRPC.
// These should do something! :-)

func (ss *Storageserver) iGet(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	//fmt.Println("Client called Get")
	node, server, correct := ss.checkServer(args.Key)

	if correct == false {
		err := server.Call("StorageRPC.Get", args, reply)
		if err != nil {
			<- ss.deadNodeListM
			ss.deadNodeList = append(ss.deadNodeList, node)
			ss.deadNodeListM <- 1
			ss.collectBodies()
			return ss.Get(args, reply)
		}
		return nil
	}

	if args.Username == "" {
		<- ss.userMapM
		val, ok := ss.userMap[args.Key]
		if ok != true {
			reply.Status = storageproto.EKEYNOTFOUND
			reply.JSONFile = ""
		} else {
			reply.Status = storageproto.OK
			reply.JSONFile = val
		}
		ss.userMapM <- 1
	} else {
		<- ss.fileMapM
		val, ok := ss.fileMap[args.Key]
		fileJSON, marshalErr := json.Marshal(val)
		if marshalErr != nil {
			//log.Fatal("Marshal error\n")
		}
		if ok != true {
			reply.Status = storageproto.EKEYNOTFOUND
			reply.JSONFile = ""
		} else {
			permission, hasPermission := val.Permissions[args.Username]

			if hasPermission == false || permission == storageproto.NONE {
				reply.Status = storageproto.ENOPERMISSION
				reply.JSONFile = ""
			} else {
				reply.Status = storageproto.OK
				reply.JSONFile = string(fileJSON)
			}
		}
		ss.fileMapM <- 1
	} 

	return nil
}

func (ss *Storageserver) iPut(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	//fmt.Println("Client called Put")
	node, server, correct := ss.checkServer(args.Key)

	if correct == false {
		//fmt.Println("Oh fuck.")
		err := server.Call("StorageRPC.Put", args, reply)
		if err != nil {
			<- ss.deadNodeListM
			ss.deadNodeList = append(ss.deadNodeList, node)
			ss.deadNodeListM <- 1
			ss.collectBodies()
			return ss.Put(args, reply)
		}
		return nil
	}

	if args.Username == "" {
		<- ss.userMapM
		ss.userMap[args.Key] = args.JSONFile
		ss.userMapM <- 1
	} else {
		var file storageproto.SyncFile
		fileBytes := []byte(args.JSONFile)
		unmarshalErr := json.Unmarshal(fileBytes, &file)
		if unmarshalErr != nil {
			////fmt.Println("Unmarshal error!\n")
		}

		permission, hasPermission := file.Permissions[args.Username]

		if hasPermission == false || (hasPermission == true && permission != storageproto.WRITE) {
			reply.Status = storageproto.ENOPERMISSION
			return nil
		}

		<- ss.fileMapM
		ss.fileMap[args.Key] = file
		ss.fileMapM <- 1
	}

	reply.Status = storageproto.OK
	return nil
}

func (ss *Storageserver) iDelete(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	//fmt.Println("Client called Delete")
	node, server, correct := ss.checkServer(args.Key)

	if correct == false {
		err := server.Call("StorageRPC.Delete", args, reply)
		if err != nil {
			<- ss.deadNodeListM
			ss.deadNodeList = append(ss.deadNodeList, node)
			ss.deadNodeListM <- 1
			ss.collectBodies()
			return ss.Delete(args, reply)
		}
		return nil
	}

	if args.Username == "" {
		<-ss.userMapM
		delete(ss.userMap, args.Key)
		ss.userMapM <- 1
	} else {

		<- ss.fileMapM
		file, ok := ss.fileMap[args.Key]
		ss.fileMapM <- 1

		if ok == true {
			if args.Username != file.Owner.Username {
				reply.Status = storageproto.ENOPERMISSION
				return nil
			}
		}

		<-ss.fileMapM
		delete(ss.fileMap, args.Key)
		ss.fileMapM <- 1
	}

	reply.Status = storageproto.OK
	return nil
}
