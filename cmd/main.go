package main

import (
	"fmt"
	"time"

	"github.com/julywind168/flying"
	"github.com/julywind168/flying/internal/gate"
	"github.com/julywind168/flying/server"
)

type Agent struct {
	ID       string
	NickName string
}

func (a *Agent) Started(ctx flying.ServiceCtx) {
	fmt.Printf("Agent %s started\n", a.ID)
}
func (a *Agent) Stopped(ctx flying.ServiceCtx) {}
func (a *Agent) Tick(ctx flying.ServiceCtx, dt time.Duration) {
	fmt.Printf("Agent %s tick, dt: %+v\n", a.ID, dt)
}

type PingPayload struct {
	Msg string
}

func (a *Agent) Ping(ctx flying.ServiceCtx, from string, payload PingPayload) {
	fmt.Printf("Agent %s ping from %s, payload: %+v\n", a.ID, from, payload)
}

func (a *Agent) Heartbeat(ctx flying.ServiceCtx, session *server.Session, payload any) {
	fmt.Printf("Agent %s heartbeat from %s, payload: %+v\n", a.ID, session.BaseNode.ID(), payload)
}

func main() {
	app := server.NewApp()
	app.World.Spawn("Agent.1", time.Second, &Agent{ID: "1", NickName: "Jack"})
	app.World.Spawn("Agent.2", time.Second, &Agent{ID: "2", NickName: "Lily"})
	app.AddGate(gate.NewWsGate("/", ":7777"))
	app.Run()
}
