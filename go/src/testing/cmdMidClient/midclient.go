package main

// Libclient offers the same command-line interface as storageclient,
// but it uses libstore instead.  You can use this program to test your
// libstore implementation or to query what's in the entire storage
// system.

import (
	"application/midclient"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
)

// For parsing the command line
type cmd_info struct {
	cmdline string
	nargs   int // number of required args
}

const (
	CMD_PUT = iota
	CMD_GET
)

var username *string = flag.String("user", "kechow", "user who is calling this function")
var portnum *int = flag.Int("port", 9010, "port for this midclient to start on")
var serverAddress *string = flag.String("host", "localhost:9009", "server host to connect to (e.g. localhost:9009)")
var numTimes *int = flag.Int("n", 1, "Number of times to execute the get or put.")

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s  <command>:\n", os.Args[0])
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n")
		fmt.Fprintf(os.Stderr, "   commands:  p  usr key val  (put)\n")
		fmt.Fprintf(os.Stderr, "              g  usr key      (get)\n")
		fmt.Fprintf(os.Stderr, "              d  usr key val  (delete)\n")
		fmt.Fprintf(os.Stderr, "              ts  key val  (toggle sync)\n")
	}

	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatal("Insufficient arguments to client")
	}

	cmd := flag.Arg(0)

	cmdlist := []cmd_info{
		{"p", 2},
		{"g", 1},
		{"d", 1},
		{"ts", 2},
	}

	cmdmap := make(map[string]cmd_info)
	for _, j := range cmdlist {
		cmdmap[j.cmdline] = j
	}

	ci, found := cmdmap[cmd]
	if !found {
		log.Fatal("Unknown command ", cmd)
	}
	if flag.NArg() < (ci.nargs + 1) {
		log.Fatal("Insufficient arguments for ", cmd)
	}
	ls, err := midclient.NewMidClient(*serverAddress, net.JoinHostPort("localhost", fmt.Sprintf("%d", *portnum)))
	if err != nil {
		log.Fatal("Could not create a midclient")
	}

	for i := 0; i < *numTimes; i++ {
		switch cmd {
		case "g", "d", "ts":
			switch cmd {
			case "g":
				val, err := ls.Get(flag.Arg(1), *username)
				if err != nil {
					fmt.Println("error: ", err)
				} else if val == "" {
					fmt.Println("There is no data at this key!")
				} else {
					fmt.Println(val)
				}
			case "d":
				err := ls.Delete(flag.Arg(1), *username)
				if err != nil {
					fmt.Println("error: ", err)
				} else {
					fmt.Println("Deleted ", flag.Arg(1))
				}
			case "ts":
				err := ls.ToggleSync(flag.Arg(1))
				if err != nil {
					fmt.Println("error: ", err)
				}
			}
		case "p":
			err := ls.Put(flag.Arg(1), flag.Arg(2), *username)
			switch err {
			case nil:
				fmt.Println("OK")
			default:
				fmt.Println("Error: ", err)
			}

		}
	}
}
