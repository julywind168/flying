package proto

type (
	LoginPayload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	RegisterPayload struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	Result struct {
		Code    ErrCode `json:"code"`              // 错误码，0 表示成功，其他表示错误
		Message string  `json:"message,omitempty"` // 错误信息
		Token   string  `json:"token,omitempty"`   // 登录成功时返回
	}
)
