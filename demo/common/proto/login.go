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
)
