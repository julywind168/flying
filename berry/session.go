package berry

import (
	"encoding/json"

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
	Response(result any)
	Push(name string, payload any)
	PushEphemeral(name string, payload any)
	GetPacket() *Packet
	SetPacket(packet *Packet)
	GetPeer() Peer
	SetPeer(peer Peer)
	// flying.Entity
	ID() string
	Lockfree() bool
	Dependencies() []string
}

type BaseSession struct {
	userID      string
	peer        Peer
	packet      *Packet // current request packet
	handshakeID uint32  // handshake unique id
	index       uint32  // server send packet index (start from 1)
	pktCache    deque.Deque[PktCacheItem]
	logger      flying.Logger
}

var _ Session = (*BaseSession)(nil)

func NewBaseSession(userID string, peer Peer, logger flying.Logger) *BaseSession {
	s := &BaseSession{
		userID:      userID,
		peer:        peer,
		handshakeID: 1,
		logger:      logger,
	}
	return s
}

func (s *BaseSession) ID() string {
	return s.userID
}

func (s *BaseSession) Lockfree() bool {
	return false
}

func (s *BaseSession) Dependencies() []string {
	return nil
}

func (s *BaseSession) send(packet Packet, noCache bool) {
	bytes, _ := json.Marshal(packet)
	if noCache {
		s.peer.Write(bytes)
		return
	}
	s.index++
	packet.Index = s.index
	s.pktCache.PushBack(PktCacheItem{
		index:  s.index,
		packge: bytes,
	})
	if s.pktCache.Len() > PKT_CACHE_SIZE {
		s.pktCache.PopFront()
	}
	s.peer.Write(bytes)
}

// idx == 0: 重新登陆
// idx > 0: 重连
func (s *BaseSession) Reconnect(id uint32, idx uint32, peer Peer) bool {
	if idx == 0 {
		s.handshakeID = 1
		s.pktCache.Clear()
		return true
	} else {
		if id != s.handshakeID {
			s.logger.Warnf("HandshakeID mismatch: %d != %d", id, s.handshakeID)
			return false
		}
		if s.pktCache.Len() < 0 {
			s.logger.Warnf("Invalid handshake packet idx, cache is empty: %d", idx)
			return false
		}
		if idx < s.pktCache.Front().index { // 缓存未命中 (客户端需重新登陆)
			return false
		}
		if idx > s.pktCache.Back().index { // 非法 index
			s.logger.Warnf("Invalid handshake packet idx: %d, cache max index %d", idx, s.pktCache.Back().index)
			return false
		}
		// 重发
		go func() {
			for i := 0; i < s.pktCache.Len(); i++ {
				pkt := s.pktCache.At(i)
				if pkt.index > idx {
					peer.Write(pkt.packge)
				}
			}
		}()
		s.handshakeID++
		return true
	}
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
		}, false)
		s.packet = nil
	} else {
		s.logger.Errorf("Session.Response: you maybe already responded")
	}
}

func (s *BaseSession) Push(name string, payload any) {
	data, _ := json.Marshal(payload)
	s.send(Packet{
		Type:    PacketTypeRequest,
		Session: 0,
		Name:    name,
		Payload: data,
	}, false)
}

// 发送消息但不加入重发缓存。适用于旧连接顶号通知等无需重发的场景。
func (s *BaseSession) PushEphemeral(name string, payload any) {
	data, _ := json.Marshal(payload)
	s.send(Packet{
		Type:    PacketTypeRequest,
		Session: 0,
		Name:    name,
		Payload: data,
	}, true)
}

func (s *BaseSession) SetPacket(packet *Packet) {
	s.packet = packet
}

func (s *BaseSession) GetPacket() *Packet {
	return s.packet
}

func (s *BaseSession) GetPeer() Peer {
	return s.peer
}

func (s *BaseSession) SetPeer(peer Peer) {
	s.peer = peer
}
