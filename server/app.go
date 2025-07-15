package server

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/julywind168/flying"
)

type App struct {
	World  *flying.World
	Gates  []IGate
	events chan any
}

type GateEventConnect struct {
	Peer IPeer
}

type GateEventDisconnect struct {
	Peer IPeer
}

type GateEventMessage struct {
	Peer IPeer
	Msg  []byte
}

func NewApp() *App {
	app := &App{
		World:  flying.NewWorld(),
		events: make(chan any, 1000),
	}
	return app
}

func (app *App) verify(peer IPeer, msg []byte) *Session {
	return nil
}

func (app *App) onConnect(peer IPeer) {
	Sugar.Infof("Connected from %s\n", peer.Address())
}

func (app *App) OnConnect(peer IPeer) {
	app.events <- GateEventConnect{Peer: peer}
}

func (app *App) onMessage(peer IPeer, msg []byte) {
	if session := peer.IsVerified(); session != nil {
		app.World.FireClientRequest(session.agent, session, "Request", msg)
	} else {
		if session := app.verify(peer, msg); session != nil {
			peer.Verified(session)
		} else {
			Sugar.Errorf("Verify error\n")
			peer.Close()
		}
	}
}

func (app *App) OnMessage(peer IPeer, msg []byte) {
	app.events <- GateEventMessage{
		Msg:  msg,
		Peer: peer,
	}
}

func (app *App) onDisconnect(peer IPeer) {
}

func (app *App) OnDisconnect(peer IPeer) {
	app.events <- GateEventDisconnect{
		Peer: peer,
	}
}

func (app *App) AddGate(gate IGate) *App {
	app.Gates = append(app.Gates, gate)
	return app
}

func (app *App) Run() {
	app.World.Start()
	for _, gate := range app.Gates {
		gate.Start(app)
	}
	defer app.cleanup()
	// Start event loop in a separate goroutine
	go func() {
		for event := range app.events {
			switch event := event.(type) {
			case GateEventConnect:
				app.onConnect(event.Peer)
			case GateEventMessage:
				app.onMessage(event.Peer, event.Msg)
			case GateEventDisconnect:
				app.onDisconnect(event.Peer)
			}
		}
	}()

	// Listen for Ctrl+C/D to stop the server gracefully
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh
	Sugar.Infof("Received interrupt, shutting down...")
}

func (app *App) cleanup() {
	for _, gate := range app.Gates {
		gate.Stop()
	}
	app.World.Stop()
}
