package demo

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/julywind168/flying/server"
)

type Authorization struct {
	Token string
}

func Verify(app *server.App, peer server.IPeer, msg []byte) (bool, server.ISession) {
	var auth Authorization
	if err := json.Unmarshal(msg, &auth); err == nil {
		uid := "1" // TODO: fix it
		agent := fmt.Sprintf("SessionAgent.%s", uid)
		app.World.Spawn(agent, 10*time.Second, &server.SessionAgent{})
		return true, server.NewSession(uid, agent, peer)
	} else {
		return false, nil
	}
}
