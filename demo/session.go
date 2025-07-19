package demo

import (
	"github.com/julywind168/flying/demo/model"
	"github.com/julywind168/flying/server"
)

type Session struct {
	server.Session
	User *model.User
}
