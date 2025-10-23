package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gorilla/websocket"
	"github.com/julywind168/flying/demo/common/proto"
)

type Result proto.LoginResult

type PacketType uint8

const (
	PacketTypeRequest PacketType = iota
	PacketTypeResponse
)

type Packet struct {
	Service string // route
	Type    PacketType
	Session uint32 // request ID
	Name    string // method name
	Payload []byte
	Index   uint32 // server packet index
}

const (
	USERNAME = "testclient"
	PASSWORD = "123456"
)

type Client struct {
	Session     uint32
	Conn        *websocket.Conn
	Token       string // handshake secret
	HandshakeId uint32
	PktIndex    uint32 // server packet index
}

func (c *Client) Handshake() {
	c.HandshakeId += 1
	handshake, _ := json.Marshal(proto.Handshake{
		Id:       c.HandshakeId,
		Token:    c.Token,
		PktIndex: c.PktIndex,
	})

	err := c.Conn.WriteMessage(websocket.TextMessage, handshake)
	if err != nil {
		panic("write: " + err.Error())
	}
	// 接收消息
	c.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, message, err := c.Conn.ReadMessage()
	if err != nil {
		panic("read: " + err.Error())
	}
	if string(message) == "200 OK" {
		fmt.Println("Handshake: 200 OK")
	} else {
		panic("Handshake error: " + string(message))
	}
}

func (c *Client) Cleanup() {
	if c.Conn != nil {
		c.Conn.Close()
	}
}

func (c *Client) SendRequest(service, name string, payload any) {
	c.Session += 1
	data, _ := json.Marshal(payload)
	p := Packet{
		Service: service,
		Type:    PacketTypeRequest,
		Session: c.Session,
		Name:    name,
		Payload: data,
	}
	if err := c.Conn.WriteJSON(p); err != nil {
		fmt.Printf("Sent request error: %+v\n", err)
		return
	} else {
		fmt.Printf("Sent request %s\n", p.Name)
	}
}

// 读取 ws 消息并打印
func (c *Client) ReadLoop() {
	for {
		var p Packet
		err := c.Conn.ReadJSON(&p)
		if err != nil {
			return
		}
		if p.Index > c.PktIndex {
			c.PktIndex = p.Index
		}
		fmt.Printf("Received: %+v\n", p)
	}
}

func Start() {
	doRegister()
	r := doLogin()
	if r.Code != proto.ErrCodeSuccess {
		panic("Login failed")
	}
	client := connectGameServer(r.Token)
	defer client.Cleanup()
	go client.ReadLoop()

	var agent = "Agent." + r.UserID

	client.SendRequest(agent, "Heartbeat", proto.HeartbeatRequest{Timestamp: time.Now().Unix()})
	// client.SendRequest(agent, "Heartbeat", proto.HeartbeatRequest{Timestamp: time.Now().Unix()})

	time.Sleep(time.Second * 3)
	println("Bye")
}

// step 1: register
func doRegister() {
	var r = jsonPost[Result]("http://127.0.0.1:9999/register", proto.RegisterRequest{
		Username: USERNAME,
		Password: PASSWORD,
	})
	fmt.Printf("Register result: %+v\n", r)
}

// step 2: login
func doLogin() Result {
	var r = jsonPost[Result]("http://127.0.0.1:9999/login", proto.LoginRequest{
		Username: USERNAME,
		Password: PASSWORD,
	})
	fmt.Printf("Login result: %+v\n", r)
	return r
}

// setp 3: connect game server
func connectGameServer(token string) *Client {
	u := url.URL{Scheme: "ws", Host: "127.0.0.1:8888", Path: ""}

	fmt.Printf("Connecting to %s\n", u.String())

	dialer := websocket.DefaultDialer
	conn, _, err := dialer.Dial(u.String(), nil)
	if err != nil {
		panic("dial: " + err.Error())
	}
	client := &Client{
		Conn:  conn,
		Token: token,
	}
	client.Handshake()
	return client
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

func main() {
	Start()
}
