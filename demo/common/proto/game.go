package proto

type HeartbeatRequest struct {
	Timestamp int64 `json:"timestamp"` // client timestamp
}

type HeartbeatResponse struct {
	Code      ErrCode `json:"code"`
	Timestamp int64   `json:"timestamp"` // server timestamp
}
