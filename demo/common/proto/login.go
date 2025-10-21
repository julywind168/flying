package proto

type (
	LoginRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	RegisterRequest struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	LoginResult struct {
		Code    ErrCode `json:"code"`              // 错误码，1 表示成功，其他表示错误
		Message string  `json:"message,omitempty"` // 错误信息
		Token   string  `json:"token,omitempty"`   // 登录成功时返回
		UserID  string  `json:"user_id,omitempty"`
	}
	RegisterResult = LoginResult
)
