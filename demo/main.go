package demo

import (
	"github.com/julywind168/flying/demo/game"
	"github.com/julywind168/flying/demo/login"
)

func Start() {
	go login.Start()
	game.Start()
}
