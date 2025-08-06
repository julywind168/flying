package proto

type Handshake struct {
	Token    string `json:"token"`     // jwt token
	Id       uint32 `json:"id"`        // handshake id (incr it each handshake, start from 1)
	PktIndex uint32 `json:"pkt_index"` // server packet index
}
