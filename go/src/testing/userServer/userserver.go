package main

import (
	"application/userclient"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"
)

var portnum *int = flag.Int("port", 9010, "port # to listen on")
var homedir *string = flag.String("homedir", "~/whiteboard", "Path to the whiteboard home directory")

func main() {
	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatal("usage:  userclient <my host port> <home dir>")
	}
	l, e := net.Listen("tcp", fmt.Sprintf(":%d", *portnum))
	if e != nil {
		log.Fatal("listen error:", e)
	}
	log.Printf("Server starting on port %d\n", *portnum)
	ts := userclient.NewUserClient(flag.Arg(0), flag.Arg(1))
	if ts != nil {
		rpc.Register(ts)
		rpc.HandleHTTP()
		http.Serve(l, nil)
	} else {
		log.Printf("Server could not be created\n")
	}
}
