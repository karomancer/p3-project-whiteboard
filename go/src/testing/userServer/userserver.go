package main

import (
	"application/userclient"
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"protos/userproto"
	"strconv"
	"strings"
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

var portnum *int = flag.Int("port", 9009, "server port # to connect to")
var homedir *string = flag.String("homedir", "whiteboard", "Home directory of whiteboard")
var persistent *bool = flag.Bool("persistent", true, "Whether to keep the user client persistent (versus one instance per command)")

func main() {
	flag.Parse()

	serverPort := fmt.Sprintf(":%d", *portnum)
	client := userclient.NewUserClient(serverPort, "localhost:"+strconv.Itoa(*portnum), *homedir)

	cmdlist := []cmd_info{
		{"uc", "UserClient.CreateUser", 3},
		{"au", "UserClient.AuthenticateUser", 2},
		{"s", "UserClient.Sync", 2},
		{"ts", "UserClient.ToggleSync", 1},
		{"ep", "UserClient.EditPermissions", 3},
		{"cc", "UserClient.MakeClass", 1},
	}

	cmdmap := make(map[string]cmd_info)
	for _, j := range cmdlist {
		cmdmap[j.cmdline] = j
	}

	for *persistent {
		var cmd []byte
		reader := bufio.NewReader(os.Stdin)
		cmd, _, scanErr := reader.ReadLine()
		if scanErr == nil {
			cmdarray := strings.Split(string(cmd), " ")

			ci, _ := cmdmap[cmdarray[0]]

			switch cmdarray[0] {
			case "uc": // user create
				if len(cmdarray) != 4 {
					fmt.Println("Insufficient arguments to User Create.\nCorrect usage: uc <username> <password> <email address>\n")
					break
				}
				logged := client.IsLoggedIn()
				if logged != "" {
					fmt.Printf("You're already logged in as %v !", logged)
					break
				}
				// oh, Go.  If only you let me do this right...
				reply := &userproto.CreateUserReply{}
				err := client.CreateUser(&userproto.CreateUserArgs{cmdarray[1], cmdarray[2], cmdarray[3]}, reply)
				PrintStatus(ci.funcname, reply.Status, err)
			case "au": //user authenticate
				if len(cmdarray) != 3 {
					fmt.Println("Insufficient arguments to Authenticate User.\nCorrect usage: au <username> <password>\n")
					break
				}
				logged := client.IsLoggedIn()
				if logged != "" {
					fmt.Printf("You're already logged in as %v !", logged)
					break
				}
				reply := &userproto.AuthenticateUserReply{}
				err := client.AuthenticateUser(&userproto.AuthenticateUserArgs{cmdarray[1], cmdarray[2]}, reply)
				if err != nil {
					log.Fatal("Authorization error. Invalid User.")
				}
				PrintStatus(ci.funcname, reply.Status, err)
			case "s": //sync
				//err := client.Sync(flag.Arg(1))
				//PrintStatus(ci.funcname, reply.Status, err)
			case "cc": //make class / class create
				if len(cmdarray) != 2 {
					fmt.Println("Insufficient arguments to Class Create.\n")
					break
				}
				logged := client.IsLoggedIn()
				if logged == "" {
					fmt.Println("You're not logged in! Please login.")
					break
				}
				reply := &userproto.MakeClassReply{}
				err := client.MakeClass(&userproto.MakeClassArgs{cmdarray[1]}, reply)
				PrintStatus(ci.funcname, reply.Status, err)
			case "ts": //toggle sync
				if len(cmdarray) != 2 {
					fmt.Println("Insufficient arguments to Toggle Sync.\nCorrect usage: ts <filename>")
					break
				}
				logged := client.IsLoggedIn()
				if logged == "" {
					fmt.Println("You're not logged in! Please login.")
					break
				}
				reply := &userproto.ToggleSyncReply{}
				err := client.ToggleSync(&userproto.ToggleSyncArgs{cmdarray[1]}, reply)
				PrintStatus(ci.funcname, reply.Status, err)
			case "ep": //edit permissions
				if len(cmdarray) > 3 {
					fmt.Println("Insufficient arguments to Edit Permissions.\nCorrect usage: ep <file> <permissions to be set> <user1> <user2> ...")
					break
				}
				logged := client.IsLoggedIn()
				if logged == "" {
					fmt.Println("You're not logged in! Please login.")
					break
				}
				permission, convErr := strconv.Atoi(cmdarray[2])
				if convErr != nil {
					log.Fatal("The second argument must be the permissions")
				}
				reply := &userproto.EditPermissionsReply{}
				users := []string{}
				for i := 4; i < len(cmdarray); i++ {
					users = append(users, cmdarray[i])
				}
				err := client.EditPermissions(&userproto.EditPermissionsArgs{cmdarray[1], permission, users}, reply)
				PrintStatus(ci.funcname, reply.Status, err)
			default:
				fmt.Println("Not a valid command.")
			}
		}

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
		return "User/Class already exists"
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
