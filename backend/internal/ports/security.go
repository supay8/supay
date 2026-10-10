package ports

import (
	"context"
	"time"
)

type SecretCipher interface {
	Encrypt([]byte) (string, error)
	EncryptString(string) (string, error)
}

type CertificateStorage interface {
	Put(context.Context, string, []byte) (string, error)
	Get(context.Context, string) ([]byte, error)
	Delete(context.Context, string) error
}

type SecretHasher interface {
	Hash(string) (string, error)
	Verify(string, string) bool
}

type AccessTokenIssuer interface {
	IssueAccessToken(string, string) (string, time.Time, error)
}
