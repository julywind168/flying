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
	"github.com/julywind168/flying/demo/common/proto"
	"github.com/julywind168/flying/demo/config"
	"github.com/julywind168/flying/server"
)

func AuthenticateToken(tokenString string) (*jwt.MapClaims, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
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

const (
	HandshakeOK           = "200 OK"
	HandshakeBadRequest   = "400 Bad Request"
	HandshakeUnauthorized = "401 Unauthorized"
	HandshakeIndexExpired = "403 Index Expired"
	HandshakeUserNotFound = "404 User Not Found"
)

func Verify(app *server.App, peer server.Peer, msg []byte) (server.Session, []byte) {
	var handshake proto.Handshake
	if err := json.Unmarshal(msg, &handshake); err != nil {
		server.Sugar.Warnw("Failed to unmarshal handshake message", "error", err, "msg", string(msg))
		return nil, []byte(HandshakeBadRequest)
	}

	claims, err := AuthenticateToken(handshake.Token)
	if err != nil {
		server.Sugar.Warnw("Handshake failed", "error", err)
		return nil, []byte(HandshakeUnauthorized)
	}

	userIDAny, ok := (*claims)["user_id"]
	if !ok {
		server.Sugar.Warn("Invalid user ID in token claims")
		return nil, []byte(HandshakeUnauthorized)
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
			return nil, []byte(HandshakeUnauthorized)
		}
		userID = uint(parsed)
	default:
		server.Sugar.Warn("Unknown user ID type in token claims")
		return nil, []byte(HandshakeUnauthorized)
	}

	// load user from database
	var user model.User
	if err := db.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		server.Sugar.Warnf("User not found: %v", err)
		return nil, []byte(HandshakeUserNotFound)
	}

	agent := fmt.Sprintf("SessionAgent.%d", userID)

	// session agent
	app.World.Spawn(agent, 10*time.Second, &server.SessionAgent{})
	// user agent
	app.World.Spawn(fmt.Sprintf("Agent.%d", userID), 10*time.Second, &Agent{ID: fmt.Sprintf("%d", userID)})

	return NewSession(agent, peer, &user), []byte(HandshakeOK)
}
