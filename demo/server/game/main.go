package game

import (
	"time"

	"github.com/julywind168/flying/berry"
	"github.com/julywind168/flying/demo/common/logger"
	"github.com/julywind168/flying/demo/common/proto"
	"github.com/julywind168/flying/demo/server/game/gate"
)

type Agent struct {
	id string
}

func (a *Agent) ID() string {
	return a.id
}

func (a *Agent) Lockfree() bool {
	return false
}

func (a *Agent) Dependencies() []string {
	return []string{}
}

func (a *Agent) Heartbeat(session *Session, payload proto.HeartbeatRequest) {
	logger.Sugar.Infof("Hearbeat: %+v", payload)
	session.Response(proto.HeartbeatResponse{
		Code:      proto.ErrCodeSuccess,
		Timestamp: time.Now().Unix(),
	})
}

func Start() {
	app := berry.NewApp(logger.Sugar, Verify)
	app.AddGate(gate.NewWsGate("/", ":8888"))
	app.Run()
}
