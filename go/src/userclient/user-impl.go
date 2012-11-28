package userclient

import (
	"encoding/json"
	"midclient"
	"os"
)

type Userclient struct {
	User      userproto.User
	hostport  string
	Midclient *midclient.Midclient
}

func iNewUserclient(myhostport string) *Userclient {
	mclient, err := midclient.NewMidclient(myhostport)
	if err != nil {
		return nil
	}
	return &Midclient{hostport: myhostport, Midclient: mclient}
}

func (uc *Userclient) iCreateUser(args *userproto.CreateUserArgs, reply *userproto.CreateUserReply) error {
	if uc.User != nil {
		//User must log out first before creating another user
		reply.Status = EEXISTS
		return nil
	}
	//check to see if username already exists
	_, exists := uc.Midclient.Get(args.Username)
	//if it doesn't we are good to go
	if exists == nil {
		//make new user
		//LATER: should actually hash the password before we store it.....
		user := &User{Username: args.Username, Password: args.Password, Email: args.Email, Classes: make(map[string]string)}
		//marshal it
		userjson, marshalErr := json.Marshal(user)
		if marshalErr != nil {
			return marshalErr
		}
		//send it to the server
		createErr := uc.Midclient.Put(args.Username, userjson)
		if createErr != nil {
			return createEr
		}
		//set the current user of the session to the one we just created
		uc.User = user
		reply.Status = userproto.OK
		return nil
	}
	//otherwise the name already exists and we must chose another name
	reply.Status = userproto.EEXISTS
	return nil
}

func (uc *Userclient) iAuthenticateUser(args *userproto.AuthenticateUserArgs, reply *userproto.AuthenticateUserReply) error {
	//check if the user exists
	userJSON, exists := uc.Midclient.Get(args.Username)
	//if they do...
	if exists != nil {
		var user userproto.User
		jsonBytes := []byte(userJSON)
		//unmarshall the data
		unmarshalErr := json.Unmarshal(jsonBytes, &user)
		if unmarshalErr != nil {
			return unmarshalErr
		}
		//check if the passwords match
		//LATER: should actually just hash the password and then check the hashes
		if args.Password == user.Password {
			//if they do it's all good and we are logged in
			reply.Status = userproto.OK
			uc.User = user
		} else {
			//otherwise the password was wrong
			reply.Status = userproto.WRONGPASSWORD
		}
		return nil
	}
	//if there isn't a user by that name just return error
	reply.Status = userproto.ENOSUCHUSER
	return nil
}

//Timer for autosyncing
func (uc *Userclient) iTimer() {

}

//Things get pushed to the user automatically, but in case it's acting funny the user can also 
//manually ask for a sync
func (uc *Userclient) iSync() error {

}

//Can toggle sync (don't sync this file anymore) or sync it again!
func (uc *Userclient) iToggleSync(args *userproto.ToggleSyncArgs, reply *userproto.ToggleSyncReply) error {

}

//can change permissions on a file, but only if you are the owner of a file. On front end will be triggered by "share this file" and can choose whether they can read or write to it.
func (uc *Userclient) iEditPermissions(args *userproto.AddPermissionsArgs, reply *userproto.AddPermissionsReply) error {
	//if args.permission = nil then we are removing people from the permission list
	//also gotta check if we are adding a user to the permission list if the user actually exists
	//otherwise if the user already exists on the list we just change the permission to the new value
	//may want to change permissions to better reflect the scaling of them or something gah
}
