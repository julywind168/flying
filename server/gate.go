package server

type GateHandler interface {
	OnConnect(peer IPeer)
	OnMessage(peer IPeer, msg []byte)
	OnDisconnect(peer IPeer)
}

type Gate interface {
	Start(handler GateHandler)
	Stop()
}
