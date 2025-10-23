package game

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/julywind168/flying/berry"
	"github.com/julywind168/flying/demo/common/db"
	"github.com/julywind168/flying/demo/common/logger"
	"github.com/julywind168/flying/demo/common/model"
	"github.com/julywind168/flying/demo/common/proto"
	"github.com/julywind168/flying/demo/config"
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
	HandshakeFailed       = "403 Handshake Failed"
	HandshakeUserNotFound = "404 User Not Found"
)

func Verify(app *berry.App, peer berry.Peer, msg []byte) (berry.Session, []byte) {
	var handshake proto.Handshake
	if err := json.Unmarshal(msg, &handshake); err != nil {
		logger.Sugar.Warnw("Failed to unmarshal handshake message", "error", err, "msg", string(msg))
		return nil, []byte(HandshakeBadRequest)
	}

	claims, err := AuthenticateToken(handshake.Token)
	if err != nil {
		logger.Sugar.Warnw("Handshake failed", "error", err)
		return nil, []byte(HandshakeUnauthorized)
	}

	userIDAny, ok := (*claims)["user_id"]
	if !ok {
		logger.Sugar.Warn("Invalid user ID in token claims")
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
			logger.Sugar.Warn("Failed to parse user ID string in token claims")
			return nil, []byte(HandshakeUnauthorized)
		}
		userID = uint(parsed)
	default:
		logger.Sugar.Warn("Unknown user ID type in token claims")
		return nil, []byte(HandshakeUnauthorized)
	}

	session := app.Query(fmt.Sprintf("%d", userID))

	if session != nil {
		return reconnect(session.(*Session), peer, &handshake)
	} else {
		return login(app, userID, peer)
	}
}

func reconnect(session *Session, peer berry.Peer, handshake *proto.Handshake) (berry.Session, []byte) {
	if session.Reconnect(handshake.Id, handshake.PktIndex, peer) {
		// kick old peer
		if p := session.GetPeer(); p != nil {
			session.PushEphemeral("kick", proto.KickPayload{
				Reason: proto.KickReasonUserReplaced,
			})
			p.Close()
		}
		session.SetPeer(peer)
		return session, []byte(HandshakeOK)
	} else {
		return nil, []byte(HandshakeFailed)
	}
}

func login(app *berry.App, userID uint, peer berry.Peer) (*Session, []byte) {
	var user model.User
	if err := db.DB.Where("id = ?", userID).First(&user).Error; err != nil {
		logger.Sugar.Warnf("User not found: %v", err)
		return nil, []byte(HandshakeUserNotFound)
	}

	app.Sched.RegisterEntity(&Agent{
		id: "Agent." + strconv.FormatUint(uint64(userID), 10),
	})

	s := NewSession(peer, &user)
	app.Login(s)

	app.Sched.RegisterEntity(s)
	return s, []byte(HandshakeOK)
}
