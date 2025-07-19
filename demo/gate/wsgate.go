package gate

import (
	"github.com/julywind168/flying/server"
	"github.com/labstack/echo/v4"
	"golang.org/x/net/websocket"
)

type WsPeer struct {
	conn      *websocket.Conn
	session   server.ISession
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
		p.conn.Write(msg)
	}
}

func (p *WsPeer) Verified(session server.ISession) {
	p.session = session
}

func (p *WsPeer) IsVerified() server.ISession {
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
	Handler server.IGateHandler
}

func NewWsGate(uri string, addr string) *WsGate {
	return &WsGate{
		uri:  uri,
		addr: addr,
	}
}

func (g *WsGate) Start(handler server.IGateHandler) {
	g.Handler = handler
	go func() {
		e := echo.New()
		e.GET(g.uri, func(c echo.Context) error {
			websocket.Handler(func(ws *websocket.Conn) {
				defer ws.Close()
				peer := &WsPeer{
					conn:      ws,
					connected: true,
				}
				handler.OnConnect(peer)
				for {
					var msg []byte
					err := websocket.Message.Receive(ws, &msg)
					if err != nil {
						peer.connected = false
						handler.OnDisconnect(peer)
						break
					}
					handler.OnMessage(peer, msg)
				}
			}).ServeHTTP(c.Response(), c.Request())
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
