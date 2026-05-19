package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims — payload JWT-токена.
type Claims struct {
	UserID   int    `json:"uid"`
	Username string `json:"usr"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// Issuer создаёт и валидирует JWT с HS256.
type Issuer struct {
	secret []byte
	ttl    time.Duration
}

// NewIssuer создаёт issuer. Если secret пустой — будет ошибка при выпуске.
func NewIssuer(secret string, ttl time.Duration) *Issuer {
	return &Issuer{secret: []byte(secret), ttl: ttl}
}

// Issue выпускает JWT для пользователя. Возвращает токен и время истечения.
func (i *Issuer) Issue(userID int, username, role string) (string, time.Time, error) {
	if len(i.secret) == 0 {
		return "", time.Time{}, errors.New("jwt secret is empty")
	}
	exp := time.Now().Add(i.ttl)
	claims := Claims{
		UserID:   userID,
		Username: username,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(exp),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   fmt.Sprintf("%d", userID),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	s, err := t.SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return s, exp, nil
}

// Parse проверяет подпись и срок действия токена.
func (i *Issuer) Parse(tokenString string) (*Claims, error) {
	if len(i.secret) == 0 {
		return nil, errors.New("jwt secret is empty")
	}
	t, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return i.secret, nil
	})
	if err != nil {
		return nil, err
	}
	c, ok := t.Claims.(*Claims)
	if !ok || !t.Valid {
		return nil, errors.New("invalid token")
	}
	return c, nil
}
