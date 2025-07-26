package game

import (
	"github.com/julywind168/flying/demo/common/model"
	"github.com/julywind168/flying/server"
)

type Session struct {
	server.Session
	User *model.User
}
