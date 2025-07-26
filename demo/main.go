package demo

import (
	"github.com/julywind168/flying/demo/servers/backend"
	"github.com/julywind168/flying/demo/servers/game"
)

func Start() {
	go backend.Start()
	game.Start()
}
