package gate

import (
	"github.com/gorilla/websocket"
	"github.com/julywind168/flying/berry"
	"github.com/labstack/echo/v4"
)

var (
	upgrader = websocket.Upgrader{}
)

type WsPeer struct {
	conn      *websocket.Conn
	session   berry.Session
	connected bool
}

func (p *WsPeer) ID() string {
	return p.conn.RemoteAddr().String()
}

func (p *WsPeer) Address() string {
	return p.conn.RemoteAddr().String()
}

func (p *WsPeer) IsConnected() bool {
	return p.connected
}

func (p *WsPeer) Write(msg []byte) {
	if p.IsConnected() {
		p.conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func (p *WsPeer) Verified(session berry.Session) {
	p.session = session
}

func (p *WsPeer) IsVerified() berry.Session {
	return p.session
}

func (p *WsPeer) Close() {
	if p.IsConnected() {
		p.conn.Close()
	}
}

type WsGate struct {
	uri     string
	addr    string
	e       *echo.Echo
	Handler berry.GateHandler
}

func NewWsGate(uri string, addr string) *WsGate {
	return &WsGate{
		uri:  uri,
		addr: addr,
	}
}

func (g *WsGate) Start(handler berry.GateHandler) {
	g.Handler = handler
	go func() {
		e := echo.New()
		e.GET(g.uri, func(c echo.Context) error {
			ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
			if err != nil {
				return err
			}
			defer ws.Close()

			peer := &WsPeer{
				conn:      ws,
				connected: true,
			}
			handler.OnConnect(peer)
			for {
				_, msg, err := ws.ReadMessage()
				if err != nil {
					peer.connected = false
					handler.OnDisconnect(peer)
					break
				}
				handler.OnMessage(peer, msg)
			}
			return nil
		})
		e.Logger.Fatal(e.Start(g.addr))
		g.e = e
	}()
}
func (g *WsGate) Stop() {
	if g.e != nil {
		g.e.Close()
	}
}
