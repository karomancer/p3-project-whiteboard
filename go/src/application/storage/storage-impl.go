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
	"//fmt"
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
	nodeid  uint32 //node id for hashing
	portnum int //port number we are listening on

	nodeIndex int // index in nodeList
	nodeList  []storageproto.Node //list of all other nodes and portnumbers, SORTED
	nodeListM chan int //mutex for protecting nodelist

	deadNodeList []storageproto.Node //dead node list used for cleaning up dropped connections
	deadNodeListM chan int //correspoinding mutex

	connMap  map[uint32]*rpc.Client //map from nodeID to connection for your skiplist
	connMapM chan int 

	fileMap  map[string]storageproto.SyncFile //map to store files along with relevant data
	fileMapM chan int

	userMap map[string]string //map to store userdata, stored as json because we don't need to actually access it on this side
	userMapM chan int

	srpc *storagerpc.StorageRPC
}

//random number generator for generating a random node number
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


//create a new storage server
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

	//set portnum
	ss.portnum = portnum

	//set up all empty lists/maps in struct
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

	//if we are not provided a buddy to connect to, we are the first server, so just start
	if buddy == "" {
		ss.srpc = storagerpc.NewStorageRPC(ss)
		rpc.Register(ss.srpc)
		fmt.Println("Storage server started. My NodeID is", ss.nodeid)
		return ss
	}

	//right now we assume the given buddy node won't fail
	//in the case that it does fail and we cannot connect to the other servers, we can simply
	//provide another node instead, since all nodes have the same data
	//Ideal solution would be to have a persistant single server that is not used in the ring
	//but keeps track of all the nodes in the ring. Then the new node would contact that node first,
	//which would then find a live server in the ring for the new node to connect to
	//However this isn't really completely a distributed solution so it may not actually be ideal
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

	//again we are assuming our buddy node won't die while we are initially connecting
	////fmt.Println("Registering in New")
	err = buddyNode.Call("StorageRPC.Register", &args, &reply)
	////fmt.Println("Finished dialing in New")
	for err != nil {
		//keep trying until we are sucessful
		err = buddyNode.Call("StorageRPC.Register", &args, &reply)
		time.Sleep(time.Duration(3) * time.Second)
	}

	//now gotta get the reply and do stuff with it

	//fmt.Println("attempting to lock node list")
	<- ss.nodeListM
	//fmt.Println("locked node list")
	ss.nodeList = reply.Servers
	//fmt.Println("attempting to unlock node list")
	ss.nodeListM <- 1
	//fmt.Println("unlocked node list")

	log.Println("Successfully joined storage node cluster.")
	////fmt.Println("NodeList: ", ss.nodeList)

	//now we should tell all other servers about our existance....
	for _, node := range ss.nodeList {
		//now we do have gaurds in place to prevent blocking if one of the other nodes is dead
		if node.NodeID != ss.nodeid {
			////fmt.Println("current node: ", node.NodeID)
			nodeConnection, success := ss.dialNode(node)
			if success == true {
				//we can only register if we dialed successfully
				////fmt.Println("able to connect to node!")
				success = ss.registerWithNode(node, nodeConnection, args, reply)
				//make sure to close the connection!
				//we can only close it if we actually connected
				if success == true {
					////fmt.Println("going to close connection")
					err = nodeConnection.Close()
				}
			}
			////fmt.Println("done with node: ", node.NodeID)
		}
	}

	////fmt.Println("about to collectBodies in new")

	//in case any of the nodes on our list were actually dead we should clear them out of our list
	ss.collectBodies()

	//now we need to check if we need to take any data from already existing nodes 
	//because some of thier old data may now hash into our range instead
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
		////fmt.Println("transfering Data")
		nodeConn.Call("StorageRPC.Transfer", transferArgs, &transferReply)
		////fmt.Println("done getting data, now adding to our stores")

		//now we store the data in our maps
		//userdata
		<- ss.userMapM
		for key, val := range transferReply.UserMap {
			ss.userMap[key] = val
		}
		ss.userMapM <- 1

		//file data
		<- ss.fileMapM
		for key, val := range transferReply.FileMap {
			//have to unmarshal because we actually need to access this info
			var file storageproto.SyncFile
			fileBytes := []byte(val)
			json.Unmarshal(fileBytes, &file)

			ss.fileMap[key] = file

		}
		ss.fileMapM <- 1

	}

	////fmt.Println("done transfering data")

	//set up rpc for rpc calls
	ss.srpc = storagerpc.NewStorageRPC(ss)
	rpc.Register(ss.srpc)
	fmt.Println("Storage server started. My NodeID is", ss.nodeid)
	fmt.Printf("I am aware of the following nodes: %v\n", ss.nodeList)

	//now we shoudl calculate our buddy list
	go ss.iCalculateSkipList()

	//fmt.Println("done creating new node")

	return ss
}

//function to transfer data from one node to another because of reasons 
//(consistant hashing meaning that data may change nodes if more nodes are added)
//this is an RPC call made by a new node who is taking some of our data
func (ss *Storageserver) TransferData(args *storageproto.TransferArgs, reply *storageproto.TransferReply) error {

	//fmt.Println("called transfer data")

	//gotta first check if we even have data to transfer
	if (len(ss.userMap) == 0) && (len(ss.fileMap) == 0) {
		////fmt.Println("no data to transfer")
		return nil
	}

	//get the name of the dude we are sending data to
	toNodeID := args.ToNode.NodeID

	//need to get nodeIndex for the node we are sending to
	toNodeIndex := 0
	//fmt.Println("attempting to lock node list")
	<- ss.nodeListM
	//fmt.Println("locked node list")
	sort.Sort(byID{ss.nodeList})
	for i := 0; i < len(ss.nodeList); i++ {
		if ss.nodeList[i].NodeID == ss.nodeid {
			toNodeIndex = i
			break
		}
	}
	//fmt.Println("attempting to unlock node list")
	ss.nodeListM <- 1
	//fmt.Println("unlocked node list")

	//look through our files to find out what we need to transfer
	<- ss.fileMapM
	////fmt.Println("starting to transfer files")
	reply.FileMap = make(map[string]string)
	for key, file := range ss.fileMap {
		userclass := strings.Split(key, "?")[0]
		keyid := Storehash(userclass)
		//check key to see if it hashes to the new node
		if (keyid <= toNodeID) ||
		(keyid > ss.nodeList[len(ss.nodeList)-1].NodeID && toNodeIndex == 0) {
			//marshall the file data
			fileJSON, _ := json.Marshal(file)
			//ready it for sending
			reply.FileMap[key] = string(fileJSON)
			//delete the data from our map because we don't need it anymore
			delete(ss.fileMap, key)
		}
	}
	ss.fileMapM <- 1
	////fmt.Println("done transfering files")

	//need to do the same stuff but for users
	<- ss.userMapM
	////fmt.Println("starting to transfer user data")
	reply.UserMap = make(map[string]string)
	for key, data := range ss.userMap {
		userclass := strings.Split(key, "?")[0]
		keyid := Storehash(userclass)
		//check key hashing to new node
		if (keyid <= toNodeID) ||
		(keyid > ss.nodeList[len(ss.nodeList)-1].NodeID && toNodeIndex == 0) {
			//put data in reply map
			reply.UserMap[key] = data
			//delete data from our map
			delete(ss.userMap, key)
		}
	}
	ss.userMapM <- 1
	//fmt.Println("done transfering data")

	////fmt.Println("everything is hunky dory")

	reply.Status = storageproto.OK
	return nil
} 

//function to clean up dead connections
func (ss *Storageserver) collectBodies() {

	//fmt.Println("called CollectBodies") 

	//fmt.Println("attempting to lock dead node list")
	<- ss.deadNodeListM
	//fmt.Println("locked dead node list")

	//if there are no dead nodes to collect, don't bother
	if len(ss.deadNodeList) == 0 {
		//fmt.Println("attempting to unlock dead node list")
		ss.deadNodeListM <- 1
		//fmt.Println("unlocked dead node list")
		return
	}

	//otherwise, for each dead node
	for _, deadNode := range ss.deadNodeList {
		//fmt.Println("attempting to lock node list")
		<-ss.nodeListM
		//fmt.Println("locked node list")
		//check in the nodeList
		for index, node := range ss.nodeList {
			//if it's in the node list we better delete it
			if deadNode.NodeID == node.NodeID {
				if index < len(ss.nodeList) - 1 {
					ss.nodeList = append(ss.nodeList[:index], ss.nodeList[index + 1])
				} else {
					ss.nodeList = ss.nodeList[:index]
				}
			}
		}
		//fmt.Println("attempting to unlock node list")
		ss.nodeListM <- 1
		//fmt.Println("unlocked node list")
	}
	//clear out the dead node list
	ss.deadNodeList = []storageproto.Node{}
	//fmt.Println("attempting to unlock dead node list")
	ss.deadNodeListM <- 1
	//fmt.Println("unlocked dead node list")

	////fmt.Println("nodeList after collecting dead nodes: ", ss.nodeList)

	//recalculate our node Index
	//fmt.Println("attempting to lock node list")
	<- ss.nodeListM
	//fmt.Println("locked node list")
	sort.Sort(byID{ss.nodeList})
	for i := 0; i < len(ss.nodeList); i++ {
		if ss.nodeList[i].NodeID == ss.nodeid {
			ss.nodeIndex = i
			break
		}
	}
	//fmt.Println("attempting to unlock node list")
	ss.nodeListM <- 1
	//fmt.Println("unlocked node list")

	//recaucluate our skip list
	ss.iCalculateSkipList()

	//fmt.Println("done collecting bodies")
	return
}

//function to make connections with nodes and check for failure
func (ss *Storageserver) dialNode(node storageproto.Node) (*rpc.Client, bool) {
	//fmt.Println("called dialNode")
	////fmt.Println("dialing node: with id:", node.HostPort, node.NodeID)
	nodeClient, err := rpc.DialHTTP("tcp", node.HostPort)
	////fmt.Println("dialed once")
	//if first dial didn't work then we will try again
	if err != nil {
		////fmt.Println("dialing once didn't work, trying 5 times with 3 second wait")
		tries := 5
		for tries > 0 {
			//we will retry 5 times
			nodeClient, err = rpc.DialHTTP("tcp", node.HostPort)
			if err == nil {
				//if we did connect, break
				break
			}
			time.Sleep(time.Duration(3) * time.Second)
			tries --
		}
	}

	////fmt.Println("done tyring to connect")

	//if we couldn't connect at all we can count the node as dead and should add it to the dead node list
	if err != nil {
		////fmt.Println("could not connect, adding node to deadList")
		//fmt.Println("attempting to lock dead node list")
		<- ss.deadNodeListM
		//fmt.Println("locked dead node list")
		////fmt.Println("successfully locked deadlist")
		ss.deadNodeList = append(ss.deadNodeList, node)
		////fmt.Println("added node to deadlist")
		//fmt.Println("attempting to unlock dead node list")
		ss.deadNodeListM <- 1
		//fmt.Println("unlocked dead node list")
		////fmt.Println("unlocked deadlist mutex")

		//fmt.Println("done dialing node")

		return nil, false
	}

	////fmt.Println("able to connect!")
	//fmt.Println("done dialing node")

	return nodeClient, true
}

//function to register with other nodes and check to make sure they aren't dead
func (ss *Storageserver) registerWithNode(node storageproto.Node, servNode *rpc.Client, args storageproto.RegisterArgs, reply storageproto.RegisterReply) bool {
	//fmt.Println("called register with node")
	err := servNode.Call("StorageRPC.Register", &args, &reply)

	//fmt.Println("err: ", err)

	if err != nil {
		//if we didn't connect the first time, try again for 5 times
		tries := 5
		for tries > 0 {
			err = servNode.Call("StorageRPC.Register", &args, &reply)
			if err == nil {
				//if we did connect, break
				break
			}
			time.Sleep(time.Duration(3) * time.Second)
			tries --
		}
	}

	//if we didn't connect add the dead node to the dead node list and collect bodies
	if err != nil {
		//fmt.Println("attempting to lock dead node list")
		<- ss.deadNodeListM
		//fmt.Println("locked dead node list")
		ss.deadNodeList = append(ss.deadNodeList, node)
		//fmt.Println("attempting to unlock dead node list")
		ss.deadNodeListM <- 1
		//fmt.Println("unlocked dead node list")

		//fmt.Println("done registering with node")

		return false
	}

	//fmt.Println("done registering with node")

	return true
}

func (ss *Storageserver) iCalculateSkipList() {
	//fmt.Println("called calculate skip list")
	//first we clear out our cold skip list
	//fmt.Println("attempting to lock conn map")
	<- ss.connMapM
	//fmt.Println("sucessfully locked conn map")
	for key, _ := range ss.connMap {
		////fmt.Println("map endtry at key: ", ss.connMap[key])
		//if we can, close any open connections
		if ss.connMap[key] != nil {
			ss.connMap[key].Close()
		}	
		//remove all entries from the connMap
		delete(ss.connMap, key)
	}
	//fmt.Println("attempting to unlcok conn map")
	ss.connMapM <- 1
	//fmt.Println("sucessfully unlocked conn map")

	numNodes := len(ss.nodeList)

	jump := numNodes / 4

	if numNodes <= 6 {
		//if we have less than 6 nodes just connect to every other node
		// ////fmt.Println("nodes less than 5!")
		for _, node := range ss.nodeList {
			if node.NodeID != ss.nodeid {
				//connect to every other node that isn't us and add it to the connMap
				buddyNode, _ := ss.dialNode(node)
				<-ss.connMapM
				ss.connMap[node.NodeID] = buddyNode
				ss.connMapM <- 1
			}
		}
		go ss.collectBodies()
	} else {
		//otherwise it's math time!
		////fmt.Println("making a buddyList by math since there are more than six nodes")
		var buddyList []storageproto.Node
		index := ss.nodeIndex
		first := index - 1
		if first < 0 {
			first = -first
		}
		//calculate who should be in our skip list
		//we take the two nodes on either side of us,
		//two nodes 1/4 of the way around the circle from us,
		//and one node directly across from us 
		//(all of this is by nodeID)
		buddyList = append(buddyList, ss.nodeList[(first)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+1)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+jump)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+2*jump)%numNodes])
		buddyList = append(buddyList, ss.nodeList[(index+3*jump)%numNodes])
		////fmt.Println("figured out who is in buddyList")

		//now we actually connect to all the nodes we decided we wanted in the skip list
		for _, node := range buddyList {
			if node.NodeID != ss.nodeid {
				buddyNode, success := ss.dialNode(node)
				if success == true {
					//if we connected alright then we should add them to the connMap
					<-ss.connMapM
					ss.connMap[node.NodeID] = buddyNode
					ss.connMapM<-1 
				}
			}
		}
		go ss.collectBodies()
	}

	////fmt.Printf("I am aware of the following nodes: %v\n", ss.nodeList)
	////fmt.Printf("What I think my buddy list is %v\n", ss.connMap)

	//fmt.Println("done calculation skip list")

	return
}

// called by a new server on all other servers when it joins (RPC call)
//allows existing servers to know when a new node connects and also give the new node a list of all servers
func (ss *Storageserver) RegisterServer(args *storageproto.RegisterArgs, reply *storageproto.RegisterReply) error {
	//fmt.Println("called register server")
	//fmt.Println("attempting to lock node list")
	<- ss.nodeListM
	//fmt.Println("locked node list")
	// ////fmt.Println("aquired nodeMap lock RegisterServer")
	ok := false
	//check to see if the new node is already in our nodeList
	for _, node := range ss.nodeList {
		if node == args.ServerInfo {
			//if it does, mark as such
			ok = true
			break
		}
	}
	//fmt.Println("attempting to unlock node list")
	ss.nodeListM <- 1
	//fmt.Println("unlocked node list")
	//if it doesn't exist we have to add it to the map and to the list
	if ok != true {
		//put it in the list
		//fmt.Println("attempting to lock node list")
		<- ss.nodeListM
		//fmt.Println("locked node list")
		// ////fmt.Println("aquired nodeList lock RegisterServer")
		ss.nodeList = append(ss.nodeList, args.ServerInfo)
		//fmt.Println("attempting to unlock node list")
		ss.nodeListM <- 1
		//fmt.Println("unlocked node list")
		// ////fmt.Println("release nodeList lock RegisterServer")
	}

	//sort the nodeList and recalculate our own node index
	//fmt.Println("attempting to lock node list")
	<- ss.nodeListM
	//fmt.Println("locked node list")
	sort.Sort(byID{ss.nodeList})
	for i := 0; i < len(ss.nodeList); i++ {
		if ss.nodeList[i].NodeID == ss.nodeid {
			ss.nodeIndex = i
			break
		}
	}
		//fmt.Println("attempting to unlock node list")
	ss.nodeListM <- 1
	//fmt.Println("unlocked node list")

	//send back the list of servers
	reply.Servers = ss.nodeList

	//redo skip list
	go ss.iCalculateSkipList()

	//fmt.Println("done registering server")

	return nil
}

//hashing function for keys
func Storehash(key string) uint32 {
	hasher := fnv.New32()
	hasher.Write([]byte(key))
	return hasher.Sum32()
}

//function to check if the request for the key is actually going to the right server
//if it's not, we send it to a server in our skip list that is closer to the key than us
func (ss *Storageserver) checkServer(key string) (storageproto.Node, *rpc.Client, bool) {

	//fmt.Println("called check server")

	//get the keyID
	userclass := strings.Split(key, "?")[0]
	keyid := Storehash(userclass)

	<-ss.nodeListM
	//fmt.Println("locked nodeLIst")
	//fmt.Println("checking my nodeIndex")
	//fmt.Println("what I think node list looks like: ", ss.nodeList)
	sort.Sort(byID{ss.nodeList})
	//fmt.Println("sorted nodeList")
	//reculatulating node id
	for i := 0; i < len(ss.nodeList); i++ {
		//fmt.Println("looking for my new nodeIndex")
		if ss.nodeList[i].NodeID == ss.nodeid {
			//fmt.Println("I found my new index! It's: ", i)
			ss.nodeIndex = i
			break
		}
	}
	//fmt.Println("don't need a new nodeIndex")
	if len(ss.nodeList) < ss.nodeIndex {
		//pretty hacky. Basically if something goes horribly wrogn with too many good servers dying we designate ourselves as the correct server
		//this is PROVISIONAL, and this case barely ever happens...
		//fmt.Println("attempting to unlock node list")
		ss.nodeListM <- 1
		//fmt.Println("unlocked node list")

		//fmt.Println("done checking server")
		return storageproto.Node{}, nil, true
	}
	//fmt.Println("attempting to unlock nodelist")
	ss.nodeListM <- 1
	//fmt.Println("unlocked nodelist")


	////fmt.Printf("\nStorehash: %v\nServehash: %v\n\n", keyid, ss.nodeid)
	////fmt.Printf("What I think the node list is %v\n", ss.nodeList)
	////fmt.Printf("What I think my buddy list is %v\n", ss.connMap)
	////fmt.Printf("Node index: %v\n", ss.nodeIndex)

	//figure out who our predecessor is
	var predecessor int
	if ss.nodeIndex == 0 {
		predecessor = len(ss.nodeList) - 1
	} else {
		predecessor = ss.nodeIndex - 1
	}

	////fmt.Printf("Predecessor index:%v\n", predecessor)

	//check actual conditions to see if the key actually hashes to our server
	//a key belongs to a server if it's <= the server's node id
	//also consider wraparound
	if (keyid <= ss.nodeList[ss.nodeIndex].NodeID && ss.nodeIndex == 0) ||
		(keyid > ss.nodeList[len(ss.nodeList)-1].NodeID && ss.nodeIndex == 0) ||
		(keyid > ss.nodeList[predecessor].NodeID && keyid <= ss.nodeList[ss.nodeIndex].NodeID) {
		//fmt.Println("This is the correct server")
		//fmt.Println("done checking server")
		return storageproto.Node{}, nil, true
	}

	//fmt.Println("OHFUCK Either the wrong server or this number is at the end of the circle.")

	//if it doesn't hash to our server we've gotta send it to someone else
	for nodeId, nodeClient := range ss.connMap {
		//fmt.Println("finding another node to give it to. Checking: ", nodeId)
		if keyid < nodeId {
			//check if the other node is a good one to send it to
			for nodeId2, _ := range ss.connMap {
				if nodeId > nodeId2 && keyid < nodeId2 {
					nodeId = nodeId2
				}
			}
			//fmt.Println("nodeID of node I'm passing it to is: ", nodeId)

			nodeToSendTo := storageproto.Node{}

			//get the actual info for that node
			//fmt.Println("attempting to lock node list")
			<-ss.nodeListM
			//fmt.Println("locked node list")
			for _, node := range ss.nodeList {
				if node.NodeID == nodeId {
					nodeToSendTo = node
				}
			}
			//fmt.Println("attempting to unlock node list")
			ss.nodeListM <- 1
			//fmt.Println("unlocked node list")

			//return the node to redirect to
			//fmt.Println("done checking server")
			return nodeToSendTo, nodeClient, false
		}
	}

	//if we get down here, this means that the key hit wraparound, 
	//so we should send it to the lowest nodeId in the skiplist
	<-ss.nodeListM
	//initally set to highest node id
	lowestNodeId := ss.nodeList[len(ss.nodeList)-1].NodeID
	ss.nodeListM<-1

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

	//fmt.Println("couldn't find another guy to give it to!")
	//if we couldn't find anyone else to send to (all other nodes died or something)
	//fmt.Println("done checking server")
	return storageproto.Node{}, nil, true
}

//get
func (ss *Storageserver) iGet(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	//fmt.Println("Client called Get")
	node, server, correct := ss.checkServer(args.Key)

	//if it's the wrong server we redirect to other server provided by get server
	if correct == false {
		if server == nil {
			return nil
		}
		err := server.Call("StorageRPC.Get", args, reply)
		if err != nil {
			<- ss.deadNodeListM
			ss.deadNodeList = append(ss.deadNodeList, node)
			ss.deadNodeListM <- 1
			ss.collectBodies()
			//fmt.Println("done with get")
			return ss.Get(args, reply)
		}
		//fmt.Println("done with get")
		return nil
	}

	//if no username is provided it's user data
	if args.Username == "" {
		//lookup the key in the usermap
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
		//otherwise look up the data in the filemap
		<- ss.fileMapM
		val, ok := ss.fileMap[args.Key]
		//gotta marshal it to send
		fileJSON, marshalErr := json.Marshal(val)
		if marshalErr != nil {
			//log.Fatal("Marshal error\n")
		}
		if ok != true {
			reply.Status = storageproto.EKEYNOTFOUND
			reply.JSONFile = ""
		} else {
			//gotta check permissions to read file
			permission, hasPermission := val.Permissions[args.Username]

			if hasPermission == false || permission == storageproto.NONE {
				//if they don't have permission deny access
				reply.Status = storageproto.ENOPERMISSION
				reply.JSONFile = ""
			} else {
				//otherwise allow
				reply.Status = storageproto.OK
				reply.JSONFile = string(fileJSON)
			}
		}
		ss.fileMapM <- 1
	} 

	//fmt.Println("done with get")
	return nil
}

func (ss *Storageserver) iPut(args *storageproto.PutArgs, reply *storageproto.PutReply) error {
	//fmt.Println("Client called Put")
	node, server, correct := ss.checkServer(args.Key)

	if correct == false {
		if server == nil {
			return nil
		}
		//if we aren't the right server, redirect
		////fmt.Println("Oh fuck.")
		err := server.Call("StorageRPC.Put", args, reply)
		if err != nil {
			<- ss.deadNodeListM
			ss.deadNodeList = append(ss.deadNodeList, node)
			ss.deadNodeListM <- 1
			ss.collectBodies()
			//if server died, mark it as such, get rid of it, and try again
			//fmt.Println("done with put")
			return ss.Put(args, reply)
		}
		//fmt.Println("done with put")
		return nil
	}

	//userdata
	if args.Username == "" {
		<- ss.userMapM
		ss.userMap[args.Key] = args.JSONFile
		ss.userMapM <- 1
	} else {
		//file data
		var file storageproto.SyncFile
		fileBytes := []byte(args.JSONFile)
		unmarshalErr := json.Unmarshal(fileBytes, &file)
		if unmarshalErr != nil {
			//////fmt.Println("Unmarshal error!\n")
		}

		//gotta check permissions
		permission, hasPermission := file.Permissions[args.Username]

		if hasPermission == false || (hasPermission == true && permission != storageproto.WRITE) {
			reply.Status = storageproto.ENOPERMISSION
			//fmt.Println("done with put")
			return nil
		}

		<- ss.fileMapM
		ss.fileMap[args.Key] = file
		ss.fileMapM <- 1
	}

	reply.Status = storageproto.OK
	//fmt.Println("done with put")
	return nil
}

func (ss *Storageserver) iDelete(args *storageproto.GetArgs, reply *storageproto.GetReply) error {
	//fmt.Println("Client called Delete")
	node, server, correct := ss.checkServer(args.Key)

	//check server, redirect if wrong
	if correct == false {
		if server == nil {
			return nil
		}
		err := server.Call("StorageRPC.Delete", args, reply)
		if err != nil {
			<- ss.deadNodeListM
			ss.deadNodeList = append(ss.deadNodeList, node)
			ss.deadNodeListM <- 1
			ss.collectBodies()
			//fmt.Println("done with delete")
			return ss.Delete(args, reply)
		}
		//fmt.Println("done with delete")
		return nil
	}

	//deleting user data
	if args.Username == "" {
		<-ss.userMapM
		delete(ss.userMap, args.Key)
		ss.userMapM <- 1
	} else {
		//deleting file data
		<- ss.fileMapM
		file, ok := ss.fileMap[args.Key]
		ss.fileMapM <- 1

		if ok == true {
			//check permissions, must be owner to delete
			if args.Username != file.Owner.Username {
				reply.Status = storageproto.ENOPERMISSION
				//fmt.Println("done with delete")
				return nil
			}
		}

		<-ss.fileMapM
		delete(ss.fileMap, args.Key)
		ss.fileMapM <- 1
	}

	reply.Status = storageproto.OK
	//fmt.Println("done with delete")
	return nil
}
