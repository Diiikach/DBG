// Package auth содержит хелперы аутентификации: хэширование пароля и JWT.
package auth

import "golang.org/x/crypto/bcrypt"

// HashPassword — bcrypt-хэш пароля с дефолтной ценой.
func HashPassword(plain string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// CheckPassword сравнивает plain-пароль с bcrypt-хэшем.
// Возвращает nil, если пароль совпал.
func CheckPassword(hash, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain))
}
