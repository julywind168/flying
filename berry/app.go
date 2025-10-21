package berry

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"github.com/julywind168/flying"
)

type App struct {
	Sched    *flying.Scheduler
	Gates    []Gate
	events   chan any
	logger   flying.Logger
	verify   func(app *App, peer Peer, msg []byte) (Session, []byte)
	sessions map[string]Session
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

func NewApp(logger flying.Logger, verify func(app *App, peer Peer, msg []byte) (Session, []byte)) *App {
	app := &App{
		Sched:    flying.New(),
		Gates:    make([]Gate, 0),
		events:   make(chan any, 1000),
		logger:   logger,
		verify:   verify,
		sessions: make(map[string]Session),
	}
	return app
}

func (app *App) Query(userID string) Session {
	return app.sessions[userID]
}

func (app *App) Login(session Session) {
	app.sessions[session.ID()] = session
}

func (app *App) Logout(session Session) {
	delete(app.sessions, session.ID())
}

func (app *App) handlePeerConnect(peer Peer) {
	app.logger.Infof("Connected from %s\n", peer.Address())
}

func (app *App) handleClientRequest(session Session, packet *Packet) {
	err := app.Sched.SubmitAsync(&flying.Event{
		Type:     "client:request:" + packet.Name,
		EntityID: packet.Service,
		Handler: func(ctx flying.Context, entity flying.Entity) error {
			session.SetPacket(packet)

			v := reflect.ValueOf(entity)
			method := v.MethodByName(packet.Name)
			if !method.IsValid() {
				return fmt.Errorf("service %s does not have a method called %s", entity.ID(), packet.Name)
			}
			methodType := method.Type()
			paramType := methodType.In(1)
			var paramValue reflect.Value

			if paramType == reflect.TypeOf([]byte(nil)) { // accept raw bytes
				paramValue = reflect.ValueOf(packet.Payload)
			} else if paramType.Kind() == reflect.Interface && paramType.NumMethod() == 0 { // accept any
				paramValue = reflect.ValueOf(packet.Payload)
			} else {
				// Try to unmarshal JSON into the expected type from method signature
				paramPtr := reflect.New(paramType)
				if err := json.Unmarshal(packet.Payload, paramPtr.Interface()); err != nil {
					return fmt.Errorf("service %s method %s: failed to unmarshal params: %s", entity.ID(), packet.Name, err.Error())
				}
				paramValue = paramPtr.Elem()
			}
			args := []reflect.Value{
				reflect.ValueOf(session),
				paramValue,
			}
			if !reflect.TypeOf(session).AssignableTo(methodType.In(0)) {
				return fmt.Errorf("service %s method %s does not have the correct signature", entity.ID(), packet.Name)
			}
			method.Call(args)
			return nil
		},
	})
	if err != nil {
		app.logger.Errorf("Error handling packet: %+v", err)
	}
}

func (app *App) handlePeerMessage(peer Peer, msg []byte) {
	if session := peer.IsVerified(); session != nil {
		var packet Packet
		json.Unmarshal(msg, &packet)
		app.logger.Infof("Received message from %s: %+v", peer.ID(), packet)
		go app.handleClientRequest(session, &packet)
	} else {
		if session, response := app.verify(app, peer, msg); session != nil {
			peer.Verified(session)
			peer.Write(response)
		} else {
			app.logger.Errorf("Verify error: %s\n", response)
			peer.Write(response)
			peer.Close()
		}
	}
}

func (app *App) handlePeerDisconnect(peer Peer) {
	app.logger.Infof("Disconnect from %s\n", peer.Address())
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
	app.Sched.Start()
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
	app.logger.Infof("Received interrupt, shutting down...")
}

func (app *App) cleanup() {
	for _, gate := range app.Gates {
		gate.Stop()
	}
	app.Sched.Stop(time.Second * 5)
}
