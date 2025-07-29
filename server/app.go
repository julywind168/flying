package server

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/julywind168/flying"
)

type App struct {
	World  *flying.World
	Gates  []Gate
	events chan any
	verify func(app *App, peer Peer, msg []byte) (Session, error)
}

type GateEventConnect struct {
	Peer Peer
}

type GateEventDisconnect struct {
	Peer Peer
}

type GateEventMessage struct {
	Peer Peer
	Msg  []byte
}

func NewApp(verify func(app *App, peer Peer, msg []byte) (Session, error)) *App {
	app := &App{
		World:  flying.NewWorld(),
		Gates:  make([]Gate, 0),
		events: make(chan any, 1000),
		verify: verify,
	}
	return app
}

func (app *App) handlePeerConnect(peer Peer) {
	Sugar.Infof("Connected from %s\n", peer.Address())
}

func (app *App) handlePeerMessage(peer Peer, msg []byte) {
	if session := peer.IsVerified(); session != nil {
		app.World.FireClientRequest(session.Agent(), session, "Request", msg)
	} else {
		if session, err := app.verify(app, peer, msg); err == nil {
			peer.Verified(session)
			peer.Write([]byte("200 OK"))
		} else {
			Sugar.Errorf("Verify error: %s\n", err.Error())
			peer.Write([]byte(err.Error()))
			peer.Close()
		}
	}
}

func (app *App) handlePeerDisconnect(peer Peer) {
	Sugar.Infof("Disconnect from %s\n", peer.Address())
}

func (app *App) OnConnect(peer Peer) {
	app.events <- GateEventConnect{Peer: peer}
}

func (app *App) OnMessage(peer Peer, msg []byte) {
	app.events <- GateEventMessage{
		Msg:  msg,
		Peer: peer,
	}
}

func (app *App) OnDisconnect(peer Peer) {
	app.events <- GateEventDisconnect{
		Peer: peer,
	}
}

func (app *App) AddGate(gate Gate) *App {
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
				app.handlePeerConnect(event.Peer)
			case GateEventMessage:
				app.handlePeerMessage(event.Peer, event.Msg)
			case GateEventDisconnect:
				app.handlePeerDisconnect(event.Peer)
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
