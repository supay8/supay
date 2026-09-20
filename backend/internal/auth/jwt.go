package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var ErrInvalidToken = errors.New("token de acceso inválido o expirado")

type Claims struct {
	Email string `json:"email"`
	Type  string `json:"type"`
	jwt.RegisteredClaims
}

type JWTManager struct {
	secret []byte
	issuer string
	ttl    time.Duration
	now    func() time.Time
}

func NewJWTManager(secret, issuer string, ttl time.Duration) *JWTManager {
	if strings.TrimSpace(issuer) == "" {
		issuer = "supay"
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	return &JWTManager{secret: []byte(secret), issuer: issuer, ttl: ttl, now: time.Now}
}

func (m *JWTManager) IssueAccessToken(userID, email string) (string, time.Time, error) {
	if len(m.secret) < 32 {
		return "", time.Time{}, errors.New("JWT_SECRET debe tener al menos 32 caracteres")
	}
	now := m.now().UTC()
	expiresAt := now.Add(m.ttl)
	claims := Claims{
		Email: email,
		Type:  "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			Audience:  jwt.ClaimStrings{"supay-frontend"},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	signed, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
	return signed, expiresAt, err
}

// VerifyAccessToken valida firma, algoritmo, issuer, audience, expiración y el
// tipo del token. Devuelve exclusivamente el subject (user_id).
func (m *JWTManager) VerifyAccessToken(raw string) (string, error) {
	if len(m.secret) < 32 {
		return "", ErrInvalidToken
	}
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("algoritmo JWT inesperado: %s", token.Method.Alg())
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer(m.issuer), jwt.WithAudience("supay-frontend"), jwt.WithLeeway(30*time.Second))
	if err != nil || !token.Valid || claims.Type != "access" || strings.TrimSpace(claims.Subject) == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
