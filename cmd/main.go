package main

import (
	"fmt"
	"time"

	"github.com/julywind168/flying"
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
	if a.ID == "2" {
		ctx.Send("Agent.1", "Ping", PingPayload{Msg: "hello world"})
	}
}

type PingPayload struct {
	Msg string
}

func (a *Agent) Ping(ctx flying.ServiceCtx, from string, payload PingPayload) {
	fmt.Printf("Agent %s ping from %s, payload: %+v\n", a.ID, from, payload.Msg)
}

func (a *Agent) Heartbeat(ctx flying.ServiceCtx, session *server.Session, payload any) {
	fmt.Printf("Agent %s heartbeat from %s, payload: %+v\n", a.ID, session.BaseNode.ID(), payload)
}

func main() {
	world := flying.NewWorld()
	world.Spawn("Agent.1", time.Second, &Agent{ID: "1", NickName: "Jack"})
	world.Spawn("Agent.2", time.Second, &Agent{ID: "2", NickName: "Lily"})
	world.Start()
	world.FireClientRequest("Agent.1", &server.Session{
		BaseNode: *flying.NewBaseNode("Session.1"),
	}, "Heartbeat", 1)
	time.Sleep(time.Second * 3)
	world.Stop()
}
