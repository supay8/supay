package auth

import (
	"github.com/brandsrx/supay/internal/ports"
	"golang.org/x/crypto/bcrypt"
)

type BcryptHasher struct{}

var _ ports.SecretHasher = BcryptHasher{}

func (BcryptHasher) Hash(secret string) (string, error) {
	data, err := bcrypt.GenerateFromPassword([]byte(secret), bcrypt.DefaultCost)
	return string(data), err
}
func (BcryptHasher) Verify(secret, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(secret)) == nil
}
