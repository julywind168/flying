package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julywind168/flying/demo/common/db"
	"github.com/julywind168/flying/demo/common/model"
	"github.com/julywind168/flying/demo/config"
	"github.com/julywind168/flying/server"
)

type Authorization struct {
	Token string `json:"token"`
}

func AuthenticateToken(tokenString string) (*jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(config.JWTSecretKey), nil
	})

	if err != nil {
		return nil, fmt.Errorf("token parsing failed: %v", err)
	}

	// 验证 token 是否有效
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		// 检查 token 是否过期
		if exp, ok := claims["exp"].(float64); ok {
			if time.Now().Unix() > int64(exp) {
				return nil, errors.New("token has expired")
			}
		} else {
			return nil, errors.New("invalid expiration claim")
		}
		return &claims, nil
	}

	return nil, errors.New("invalid token")
}

/*
Handshake Response

	200 OK
	400 Bad Request
	401 Unauthorized
	403 Index Expired
	404 User Not Found
*/
func Verify(app *server.App, peer server.Peer, msg []byte) (server.Session, error) {
	var auth Authorization
	if err := json.Unmarshal(msg, &auth); err != nil {
		server.Sugar.Warnw("Failed to unmarshal authorization message", "error", err, "msg", string(msg))
		return nil, fmt.Errorf("400 Bad Request")
	}

	claims, err := AuthenticateToken(auth.Token)
	if err != nil {
		server.Sugar.Warnw("Authentication failed", "error", err)
		return nil, fmt.Errorf("401 Unauthorized")
	}

	userIDAny, ok := (*claims)["user_id"]
	if !ok {
		server.Sugar.Warn("Invalid user ID in token claims")
		return nil, fmt.Errorf("401 Unauthorized")
	}

	var userID uint
	switch v := userIDAny.(type) {
	case float64:
		userID = uint(v)
	case int:
		userID = uint(v)
	case uint:
		userID = v
	case string:
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			server.Sugar.Warn("Failed to parse user ID string in token claims")
			return nil, fmt.Errorf("401 Unauthorized")
		}
		userID = uint(parsed)
	default:
		server.Sugar.Warn("Unknown user ID type in token claims")
		return nil, fmt.Errorf("401 Unauthorized")
	}

	// load user from database
	var user model.User
	if err := db.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		server.Sugar.Warnf("User not found: %v", err)
		return nil, fmt.Errorf("404 User Not Found")
	}

	agent := fmt.Sprintf("SessionAgent.%d", userID)
	app.World.Spawn(agent, 10*time.Second, &server.SessionAgent{})

	return NewSession(agent, peer, &user), nil
}
