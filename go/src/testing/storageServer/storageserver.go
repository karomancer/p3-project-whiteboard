package main

// Server driver - creates a storage server and registers it for RPC.
//
// DO NOT MODIFY THIS FILE FOR YOUR PROJECT

import (
	"application/storage" // 'official' vs 'contrib' here
	"application/storagerpc"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
	"strconv"
)

var portnum *int = flag.Int("port", 0, "port # to listen on.  Non-master nodes default to using an ephemeral port (0), master nodes default to 9009.")
var buddy *string = flag.String("buddy", "", "Address of node to connect to within cluster")
var numNodes *int = flag.Int("N", 0, "Become the master.  Specifies the number of nodes in the system, including the master.")
var nodeID *uint = flag.Uint("id", 0, "The node ID to use for consistent hashing.  Should be a 32 bit number.")

func main() {
	flag.Parse()
	l, e := net.Listen("tcp", fmt.Sprintf(":%d", *portnum))
	if e != nil {
		log.Fatal("listen error:", e)
	}
	_, listenport, _ := net.SplitHostPort(l.Addr().String())
	log.Println("Server starting on ", listenport)
	*portnum, _ = strconv.Atoi(listenport)
	ss := storage.NewStorageServer(*buddy, *portnum, uint32(*nodeID))

	srpc := storagerpc.NewStorageRPC(ss)
	rpc.Register(srpc)
	rpc.HandleHTTP()
	http.Serve(l, nil)
}
