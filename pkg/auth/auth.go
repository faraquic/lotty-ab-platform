package auth

import (
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token expired")
)

type Tokenizer interface {
	GenerateToken(userID int64, role string) (string, time.Time, error)
	ParseToken(token string) (int64, error)
}

type JWTManager struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTManager(secret string, ttl time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), ttl: ttl}
}

func (m *JWTManager) GenerateToken(userID int64, role string) (string, time.Time, error) {
	now := time.Now()

	expiry := now.Add(m.ttl)

	claims := jwt.MapClaims{
		"sub":  strconv.FormatInt(userID, 10),
		"role": role,
		"iat":  jwt.NewNumericDate(now),
		"exp":  jwt.NewNumericDate(expiry),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign token: %w", err)
	}

	return signed, expiry, nil
}

func (m *JWTManager) ParseToken(token string) (int64, error) {
	var claims jwt.RegisteredClaims

	parsed, err := jwt.ParseWithClaims(token, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	})
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return 0, ErrExpiredToken
		}
		return 0, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	if !parsed.Valid {
		return 0, ErrInvalidToken
	}

	userID, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil || userID < 1 {
		return 0, ErrInvalidToken
	}

	return userID, nil
}
