package berry

type GateHandler interface {
	OnConnect(peer Peer)
	OnMessage(peer Peer, msg []byte)
	OnDisconnect(peer Peer)
}

type Gate interface {
	Start(handler GateHandler)
	Stop()
}
