package main

import (
	"application/userclient"
	"flag"
	"fmt"
	"log"
	"protos/userproto"
	"strconv"
	//"P2-f12/official/tribbleclient"
	//"time"
)

// For parsing the command line
type cmd_info struct {
	cmdline  string
	funcname string
	nargs    int // number of required args
}

const (
	CMD_PUT = iota
	CMD_GET
)

var portnum *int = flag.Int("port", 9010, "server port # to connect to")

func main() {

	flag.Parse()
	if flag.NArg() < 2 {
		log.Fatal("Insufficient arguments to client")
	}

	cmd := flag.Arg(0)

	serverPort := fmt.Sprintf(":%d", *portnum)
	client := userclient.NewUserclient(serverPort, "localhost:9010")

	cmdlist := []cmd_info{
		{"uc", "UserClient.CreateUser", 1},
		{"au", "UserClient.AuthenticateUser", 2},
		{"s", "UserClient.Sync", 2},
		{"ts", "UserClient.ToggleSync", 2},
		{"ep", "UserClient.EditPermissions", 3},
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

	switch cmd {
	case "uc": // user create
		// oh, Go.  If only you let me do this right...
		var reply *userproto.CreateUserReply
		err := client.CreateUser(&userproto.CreateUserArgs{flag.Arg(1), flag.Arg(2), flag.Arg(3)}, reply)
		PrintStatus(ci.funcname, reply.Status, err)
	case "au": //user authenticate
		var reply *userproto.AuthenticateUserReply
		err := client.AuthenticateUser(&userproto.AuthenticateUserArgs{flag.Arg(1), flag.Arg(2)}, reply)
		PrintStatus(ci.funcname, reply.Status, err)
	case "s": //sync
		//err := client.Sync(flag.Arg(1))
		//PrintStatus(ci.funcname, reply.Status, err)
	case "ts": //toggle sync
		var reply *userproto.ToggleSyncReply
		err := client.ToggleSync(&userproto.ToggleSyncArgs{flag.Arg(1)}, reply)
		PrintStatus(ci.funcname, reply.Status, err)
	case "ep": //edit permissions
		permission, convErr := strconv.Atoi(flag.Arg(2))
		if convErr != nil {
			log.Fatal("The second argument must be the permissions")
		}
		var reply *userproto.EditPermissionsReply
		users := []string{}
		for i := 3; i < len(flag.Args()); i++ {
			users = append(users, flag.Arg(i))
		}
		err := client.EditPermissions(&userproto.EditPermissionsArgs{flag.Arg(1), permission, users}, reply)
		PrintStatus(ci.funcname, reply.Status, err)
	}
}

// This is a little lazy, but there are only 4 entries...
func StatusToString(status int) string {
	switch status {
	case userproto.OK:
		return "OK"
	case userproto.NOTLOGGEDIN:
		return "There is no user logged in"
	case userproto.WRONGPASSWORD:
		return "Wrong password"
	case userproto.ENOSUCHUSER:
		return "No such user"
	case userproto.ENOSUCHTARGETUSER:
		return "No such target user"
	case userproto.ENOSUCHCLASS:
		return "No such class exists"
	case userproto.ENOSUCHFILE:
		return "No such file exists"
	case userproto.EEXISTS:
		return "User already exists"
	}
	return "Unknown error"
}

func PrintStatus(cmdname string, status int, err error) {
	if status == userproto.OK {
		fmt.Printf("%s succeeded\n", cmdname)
	} else {
		fmt.Printf("%s failed: %s\n", cmdname, StatusToString(status))
	}
}
