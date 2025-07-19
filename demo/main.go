package demo

import (
	"fmt"
	"time"

	"github.com/julywind168/flying"
	"github.com/julywind168/flying/demo/gate"
	"github.com/julywind168/flying/demo/model"
	"github.com/julywind168/flying/server"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
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

func Start() {
	dsn := "host=localhost user=postgres dbname=game port=5432 sslmode=disable TimeZone=Asia/Shanghai"
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic("failed to connect database")
	}

	err = db.AutoMigrate(&model.User{})
	if err != nil {
		panic("failed to migrate table")
	}

	user := &model.User{Name: "Jack", Age: 18}
	result := db.Create(&user)
	fmt.Printf("insert result: %+v\n", result)

	app := server.NewApp(Verify)
	app.World.Spawn("Agent.1", time.Second, &Agent{ID: "1", NickName: "Jack"})
	app.World.Spawn("Agent.2", time.Second, &Agent{ID: "2", NickName: "Lily"})
	app.AddGate(gate.NewWsGate("/", ":7777"))
	app.Run()
}
