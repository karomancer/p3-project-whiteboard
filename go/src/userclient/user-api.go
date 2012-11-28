package userclient

import (
	"encoding/json"
	"midclient"
	"os"
)

func NewUserclient(myhostport string) *Userclient {
	return iNewUserClient(myhostport)
}

func (uc *Userclient) CreateUser(args *userproto.CreateUserArgs, reply *userproto.CreateUserReply) error {
	return uc.iCreateUser(args, reply)
}

func (uc *Userclient) AuthenticateUser(args *userproto.AuthenticateUserArgs, reply *userproto.AuthenticateUserReply) error {
	return uc.iAuthenticateUser(args, reply)
}

//Things get pushed to the user automatically, but in case it's acting funny the user can also 
//manually ask for a sync
func (uc *Userclient) Sync(args *userproto.ToggleSyncArgs, reply *userproto.ToggleSyncReply) error {
	return uc.iSync(args, reply)
}

//Can toggle sync (don't sync this file anymore) or sync it again!
func (uc *Userclient) ToggleSync(args *userproto.ToggleSyncArgs, reply *userproto.ToggleSyncReply) error {
	return uc.iToggleSync(args, reply)
}

func (uc *Userclient) EditPermissions(args *userproto.EditPermissionsArgs, reply *userproto.EditPermissionsReply) error {
	return uc.iEditPermissions(args, reply)
}
