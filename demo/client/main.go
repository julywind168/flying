package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/julywind168/flying/demo/common/proto"
)

type Result struct {
	Code    proto.ErrCode `json:"code"`              // 错误码，1 表示成功，其他表示错误
	Message string        `json:"message,omitempty"` // 错误信息
	Token   string        `json:"token,omitempty"`   // 登录成功时返回
}

type Authorization struct {
	Token string `json:"token"`
}

const (
	USERNAME = "testclient"
	PASSWORD = "123456"
)

func Start() {
	doRegister()
	r := doLogin()
	if r.Code != proto.ErrCodeSuccess {
		panic("Login failed")
	}
	connectGameServer(r.Token)
}

// step 1: register
func doRegister() {
	var r = jsonPost[Result]("http://127.0.0.1:9999/register", proto.RegisterRequest{
		Username: USERNAME,
		Password: PASSWORD,
	})
	fmt.Printf("register result: %+v\n", r)
}

// step 2: login
func doLogin() Result {
	var r = jsonPost[Result]("http://127.0.0.1:9999/login", proto.LoginRequest{
		Username: USERNAME,
		Password: PASSWORD,
	})
	fmt.Printf("login result: %+v\n", r)
	return r
}

// setp 3: connect game server
func connectGameServer(token string) {
	u := url.URL{Scheme: "ws", Host: "127.0.0.1:8888", Path: ""}

	fmt.Printf("connecting to %s\n", u.String())

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatal("dial:", err)
	}
	defer conn.Close()

	handshake, _ := json.Marshal(Authorization{
		Token: token,
	})

	// 发送 handshake
	err = conn.WriteMessage(websocket.TextMessage, handshake)
	if err != nil {
		log.Println("write:", err)
		return
	}

	// 接收消息
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err := conn.ReadMessage()
	if err != nil {
		log.Println("read:", err)
		return
	}
	log.Printf("received: %s", message)
}

func jsonPost[T any](url string, data any) T {
	jsonBytes, _ := json.Marshal(data)
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(jsonBytes))
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result T
	json.Unmarshal(body, &result)
	return result
}
