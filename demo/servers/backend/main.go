package backend

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julywind168/flying/demo/common/db"
	"github.com/julywind168/flying/demo/common/model"
	"github.com/julywind168/flying/demo/common/proto"
	"github.com/julywind168/flying/demo/config"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// login or register result
type Result struct {
	Code    proto.ErrCode `json:"code"`              // 错误码，1 表示成功，其他表示错误
	Message string        `json:"message,omitempty"` // 错误信息
	Token   string        `json:"token,omitempty"`   // 登录成功时返回
}

func Start() {
	db := db.NewDB()

	e := echo.New()
	e.Use(middleware.Logger())
	e.Use(middleware.Recover())

	// Routes
	e.POST("/login", loginHandler(db))
	e.POST("/register", registerHandler(db))

	// Start server
	if err := e.Start(":9999"); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("failed to start server", "error", err)
	}
}

// 注册处理
func registerHandler(db *gorm.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req proto.RegisterRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusOK, Result{Code: proto.ErrCodeInvalidParams, Message: "Invalid request"})
		}

		// 检查用户名是否已存在
		var existingUser model.User
		if err := db.Where("username = ?", req.Username).First(&existingUser).Error; err == nil {
			return c.JSON(http.StatusOK, Result{Code: proto.ErrCodeUserExisted, Message: "Username already exists"})
		}

		// 加密密码
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.JSON(http.StatusOK, Result{Code: proto.ErrCodeInternal, Message: "Failed to hash password"})
		}

		// 创建用户
		user := model.User{
			Username: req.Username,
			Password: string(hashedPassword),
		}
		if err := db.Create(&user).Error; err != nil {
			return c.JSON(http.StatusOK, Result{Code: proto.ErrCodeInternal, Message: "Failed to create user"})
		}

		token, err := genToken(&user)
		if err != nil {
			return c.JSON(http.StatusOK, Result{Code: proto.ErrCodeInternal, Message: "Failed to generate token"})
		}

		return c.JSON(http.StatusOK, Result{
			Code:    proto.ErrCodeSuccess,
			Message: "User registered successfully",
			Token:   token,
		})
	}
}

// 登录处理
func loginHandler(db *gorm.DB) echo.HandlerFunc {
	return func(c echo.Context) error {
		var req proto.LoginRequest
		if err := c.Bind(&req); err != nil {
			return c.JSON(http.StatusOK, Result{Code: proto.ErrCodeInvalidParams, Message: "Invalid request"})
		}

		// 查找用户
		var user model.User
		if err := db.Where("username = ?", req.Username).First(&user).Error; err != nil {
			return c.JSON(http.StatusOK, Result{Code: proto.ErrCodeUserNotFound, Message: "User not found"})
		}

		// 验证密码
		if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
			return c.JSON(http.StatusOK, Result{Code: proto.ErrCodeUnauthorized, Message: "Invalid password"})
		}

		token, err := genToken(&user)
		if err != nil {
			return c.JSON(http.StatusOK, Result{Code: proto.ErrCodeInternal, Message: "Failed to generate token"})
		}

		return c.JSON(http.StatusOK, Result{
			Code:    proto.ErrCodeSuccess,
			Message: "Login successful",
			Token:   token,
		})
	}
}

func genToken(user *model.User) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"exp":     time.Now().Add(time.Hour * 24).Unix(),
	})
	return token.SignedString([]byte(config.JWTSecretKey))
}
