package game

import (
	"strconv"

	"github.com/julywind168/flying/demo/common/model"
	"github.com/julywind168/flying/server"
)

type Session struct {
	server.Session
	User *model.User
}

func NewSession(agent string, peer server.IPeer, user *model.User) *Session {
	return &Session{
		Session: *server.NewSession(strconv.FormatUint(uint64(user.ID), 10), agent, peer),
		User:    user,
	}
}
