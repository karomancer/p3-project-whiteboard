package userclient

import (
	"encoding/json"
	"midclient"
	"os"
)

type Userclient struct {
	User      userproto.User
	BuddyNode string
	hostport  string
	Midclient *midclient.Midclient
}

func NewUserclient(myhostport string) *Userclient {
	mclient, err := midclient.NewMidclient(myhostport)
	if err != nil {
		return nil
	}
	return &Midclient{BuddyNode: mclient.Buddy, hostport: myhostport, Midclient: mclient}
}

func (uc *Userclient) CreateUser(args *userproto.CreateUserArgs, reply *userproto.CreateUserReply) error {
	_, exists := uc.Midclient.Get(args.Username)
	user := &User{Username: args.Username, Password: args.Password, Email: args.Email, Classes: make(map[string]string)}
	if exists != nil {
		userjson, marshalErr := json.Marshal(user)
		if marshalErr != nil {
			return marshalErr
		}
		createErr := uc.Midclient.Put(args.Username, userjson)
		if createErr != nil {
			return createEr
		}
		uc.User = user
		reply.Status = userproto.OK
		return nil
	}
	reply.Status = userproto.EEXISTS
	return nil
}

func (uc *Userclient) AuthenticateUser(args *userproto.AuthenticateUserArgs, reply *userproto.AuthenticateUserReply) error {
	userJSON, exists := uc.Midclient.Get(args.Username)
	if exists != nil {
		var user userproto.User
		jsonBytes := []byte(userJSON)
		unmarshalErr := json.Unmarshal(jsonBytes, &user)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		if args.Password == user.Password {
			reply.Status = userproto.OK
			uc.User = user
		} else {
			reply.Status = userproto.WRONGPASSWORD
		}
		return nil
	}
	reply.Status = userproto.ENOSUCHUSER
	return nil
}

//Things get pushed to the user automatically, but in case it's acting funny the user can also 
//manually ask for a sync
func (uc *Userclient) Sync() error {

}

func (uc *Userclient) MakeDirectory() error {

}

func (uc *Userclient) RemoveDirectory() error {

}

func (uc *Userclient) MakeFile() error {

}

func (uc *Userclient) RemoveFile() error {

}

func (uc *Userclient) AddPermissions() error {

}

func (uc *Userclient) RemovePermissions() error {

}
