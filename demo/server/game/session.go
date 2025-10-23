package game

import (
	"strconv"

	"github.com/julywind168/flying/berry"
	"github.com/julywind168/flying/demo/common/logger"
	"github.com/julywind168/flying/demo/common/model"
)

type Session struct {
	berry.BaseSession
	User *model.User
}

func NewSession(peer berry.Peer, user *model.User) *Session {
	return &Session{
		BaseSession: *berry.NewBaseSession(strconv.FormatUint(uint64(user.ID), 10), peer, logger.Sugar),
		User:        user,
	}
}
