package server

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gammazero/deque"
	"github.com/julywind168/flying"
)

const PKT_CACHE_SIZE = 128

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

type Peer interface {
	ID() string
	Address() string
	IsConnected() bool
	Write(msg []byte)
	Verified(session Session)
	IsVerified() Session
	Close()
}

type PktCacheItem struct {
	index  uint32
	packge []byte
}

type Session interface {
	flying.Node
	Agent() string
	Response(result any)
	Push(name string, payload any)
	GetPacket() *Packet
	SetPacket(packet *Packet)
}

type BaseSession struct {
	flying.BaseNode
	agent    string // session agent service name
	peer     Peer
	packet   *Packet // current request packet
	index    uint32  // server send packet index (start from 1)
	pktCache deque.Deque[PktCacheItem]
}

var _ Session = (*BaseSession)(nil)
var _ flying.ISession = (*BaseSession)(nil)

func NewBaseSession(uid string, agent string, peer Peer) *BaseSession {
	s := &BaseSession{
		BaseNode: *flying.NewBaseNode(fmt.Sprintf("Session.%s", uid)),
		agent:    agent,
		peer:     peer,
	}
	return s
}

func (s *BaseSession) Agent() string {
	return s.agent
}

func (s *BaseSession) send(packet Packet) {
	s.index++
	packet.Index = s.index
	bytes, _ := json.Marshal(packet)
	s.pktCache.PushBack(PktCacheItem{
		index:  s.index,
		packge: bytes,
	})
	if s.pktCache.Len() > PKT_CACHE_SIZE {
		s.pktCache.PopFront()
	}
	s.peer.Write(bytes)
}

func (s *BaseSession) Response(result any) {
	if s.packet != nil {
		payload, _ := json.Marshal(result)
		s.send(Packet{
			Service: "",
			Type:    PacketTypeResponse,
			Session: s.packet.Session,
			Name:    s.packet.Name,
			Payload: payload,
		})
		s.packet = nil
	} else {
		Sugar.Errorln("Session.Response: you maybe already response")
	}
}

func (s *BaseSession) Push(name string, payload any) {
	data, _ := json.Marshal(payload)
	s.send(Packet{
		Type:    PacketTypeRequest,
		Session: 0,
		Name:    name,
		Payload: data,
	})
}

func (s *BaseSession) SetPacket(packet *Packet) {
	s.packet = packet
}

func (s *BaseSession) GetPacket() *Packet {
	return s.packet
}

type SessionAgent struct{}

var _ flying.UService = (*SessionAgent)(nil)

func (s *SessionAgent) Started(ctx flying.ServiceCtx)                {}
func (s *SessionAgent) Stopped(ctx flying.ServiceCtx)                {}
func (s *SessionAgent) Tick(ctx flying.ServiceCtx, dt time.Duration) {}
func (a *SessionAgent) Request(ctx flying.ServiceCtx, session Session, packet Packet) {
	if packet.Type == PacketTypeRequest {
		if p := session.GetPacket(); p != nil {
			Sugar.Errorf("You maybe forgot to response the request %+v\n", p)
		}
		session.SetPacket(&packet)
		ctx.FireClientRequest(packet.Service, session, packet.Name, packet.Payload)
	} else {
		Sugar.Errorln("This example doesn't support request client")
	}
}
