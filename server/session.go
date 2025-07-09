package server

import (
	"encoding/json"
	"time"

	"github.com/gammazero/deque"
	"github.com/julywind168/flying"
)

const PKG_CACHE_SIZE = 128

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
	Payload any
}

type Conn interface {
	ID() string
	IsConnected() bool
	Write(msg []byte)
}

type PkgCacheItem struct {
	index  uint32
	packge []byte
}

type Session struct {
	flying.BaseNode
	conn     Conn
	packet   Packet // current request packet
	request  bool
	index    uint32 // server send packet index (start from 1)
	pkgCache deque.Deque[PkgCacheItem]
}

var _ flying.ISession = (*Session)(nil)

func (s *Session) send(packet Packet) {
	if bytes, _ := json.Marshal(packet); bytes != nil {
		s.index++
		s.pkgCache.PushBack(PkgCacheItem{
			index:  s.index,
			packge: bytes,
		})
		if s.pkgCache.Len() > PKG_CACHE_SIZE {
			s.pkgCache.PopFront()
		}
		s.conn.Write(bytes)
	} else {
		Sugar.Errorf("packet %+v marshal error\n", packet)
	}
}

func (s *Session) Response(result any) {
	if s.request {
		s.send(Packet{
			Service: "",
			Type:    PacketTypeResponse,
			Session: s.packet.Session,
			Name:    s.packet.Name,
			Payload: result,
		})
		s.request = false
		s.packet = Packet{}
	} else {
		Sugar.Errorln("Session.Response: you maybe already response")
	}
}

func (s *Session) Push(name string, params any) {
}

type SessionAgent struct{}

var _ flying.UService = (*SessionAgent)(nil)

func (s *SessionAgent) Started(ctx flying.ServiceCtx)                {}
func (s *SessionAgent) Stopped(ctx flying.ServiceCtx)                {}
func (s *SessionAgent) Tick(ctx flying.ServiceCtx, dt time.Duration) {}
func (a *SessionAgent) Request(ctx flying.ServiceCtx, session *Session, packet Packet) {
	if packet.Type == PacketTypeRequest {
		if session.request {
			Sugar.Errorf("You maybe forgot to response the request %+v\n", session.packet)
		}
		session.packet = packet
		session.request = true
		ctx.FireClientRequest(packet.Service, session, flying.Message{Name: packet.Name, Params: packet.Payload})
	} else {
		Sugar.Errorln("This example doesn't support request client")
	}
}
