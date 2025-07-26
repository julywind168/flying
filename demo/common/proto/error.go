package proto

type ErrCode uint32

const (
	ErrCodeSuccess ErrCode = iota + 1
	ErrCodeUnknown

	// 客户端错误（1xxx）
	ErrCodeBadRequest ErrCode = 1000 + iota
	ErrCodeInvalidParams
	ErrCodeUnauthorized
	ErrCodeForbidden
	ErrCodeNotFound

	// 服务端错误（2xxx）
	ErrCodeInternal ErrCode = 2000 + iota
	ErrCodeNotImplemented
	ErrCodeServiceUnavailable

	// 业务错误（3xxx+）
	ErrCodeUserNotFound ErrCode = 3000 + iota
	ErrCodeUserExisted
)

// 错误码映射描述
var errorMessages = map[ErrCode]string{
	ErrCodeSuccess:            "success",
	ErrCodeUnknown:            "unknown error",
	ErrCodeBadRequest:         "bad request",
	ErrCodeInvalidParams:      "invalid parameters",
	ErrCodeUnauthorized:       "unauthorized",
	ErrCodeForbidden:          "forbidden",
	ErrCodeNotFound:           "not found",
	ErrCodeInternal:           "internal server error",
	ErrCodeNotImplemented:     "not implemented",
	ErrCodeServiceUnavailable: "service unavailable",
	ErrCodeUserNotFound:       "user not found",
	ErrCodeUserExisted:        "user already exists",
}

func (e ErrCode) Message() string {
	if msg, ok := errorMessages[e]; ok {
		return msg
	}
	return "unknown error"
}
