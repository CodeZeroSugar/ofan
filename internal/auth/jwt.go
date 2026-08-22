package auth

import (
	"fmt"
	"time"

	"github.com/CodeZeroSugar/ofan/internal/db"
	"github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID   int64  `json:"user_id"`
	Username string `json:"username"`
	jwt.RegisteredClaims
}

type Manager struct {
	store  *db.Store
	secret []byte
}

func NewManager(store *db.Store, secret []byte) *Manager {
	return &Manager{
		store:  store,
		secret: secret,
	}
}

func (m *Manager) IssueJWT(userID int64, username string) (string, error) {
	now := time.Now().UTC()
	claims := Claims{
		UserID:   userID,
		Username: username,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(now.Add(60 * time.Minute)),
			IssuedAt:  jwt.NewNumericDate(now),
			Issuer:    "ofan",
			Subject:   fmt.Sprintf("%d", userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secret)
}

func (m *Manager) VerifyJWT(token string) (*Claims, error) {
	claims := &Claims{}

	parsed, err := jwt.ParseWithClaims(token, claims, func(t *jwt.Token) (interface{}, error) {
		return m.secret, nil
	})
	if err != nil || !parsed.Valid {
		return nil, fmt.Errorf("invalid token: %w", err)
	}
	return claims, nil
}
