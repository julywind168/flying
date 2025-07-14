package server

import (
	"github.com/julywind168/flying"
)

type App struct {
	World *flying.World
	Gates []IGate
}

func NewApp() *App {
	app := &App{
		World: flying.NewWorld(),
	}
	return app
}

func (app *App) verify(peer IPeer, msg []byte) *Session {
	return nil
}

func (app *App) OnConnect(peer IPeer) {
	Sugar.Infof("Connected from %s\n", peer.Address())
}

func (app *App) OnMessage(peer IPeer, msg []byte) {
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

func (app *App) OnDisconnect(peer IPeer) {
}

func (app *App) AddGate(gate IGate) {
	app.Gates = append(app.Gates, gate)
}

func (app *App) Run() {
	app.World.Start()
	for _, gate := range app.Gates {
		gate.Start(app)
	}
}

func (app *App) Stop() {
	app.World.Stop()
}
