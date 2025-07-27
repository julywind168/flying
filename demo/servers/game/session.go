package game

import (
	"strconv"

	"github.com/julywind168/flying/demo/common/model"
	"github.com/julywind168/flying/server"
)

type Session struct {
	server.BaseSession
	User *model.User
}

func NewSession(agent string, peer server.Peer, user *model.User) *Session {
	return &Session{
		BaseSession: *server.NewBaseSession(strconv.FormatUint(uint64(user.ID), 10), agent, peer),
		User:        user,
	}
}
