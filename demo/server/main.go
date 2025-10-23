package main

import (
	"github.com/julywind168/flying/demo/server/backend"
	"github.com/julywind168/flying/demo/server/game"
)

func main() {
	go backend.Start()
	game.Start()
}
