package proto

type Handshake struct {
	Token    string `json:"token"`     // jwt token
	Id       uint32 `json:"id"`        // handshake id (incr it each handshake, start from 1)
	PktIndex uint32 `json:"pkt_index"` // server packet index
}

type KickReason uint32

const (
	KickReasonUnknown      KickReason = iota
	KickReasonInactive                // 1	玩家处于非活动状态
	KickReasonUserBanned              // 2	用户被封禁
	KickReasonUserReplaced            // 3	用户被顶号
)

type KickPayload struct {
	Reason KickReason `json:"reason"`
}
