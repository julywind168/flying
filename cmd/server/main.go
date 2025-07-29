package main

import (
	"github.com/julywind168/flying/demo/servers/backend"
	"github.com/julywind168/flying/demo/servers/game"
)

func main() {
	go backend.Start()
	game.Start()
}
