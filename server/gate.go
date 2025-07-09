package server

type IGateHandler interface {
	OnConnect(peer IPeer)
	OnMessage(peer IPeer, msg []byte)
	OnDisconnect(peer IPeer)
}

type IGate interface {
	Start(handler IGateHandler)
	Stop()
}
