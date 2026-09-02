package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/hkdf"
)

// Service cifra/descifra datos sensibles (token delegado, password P12, bytes P12)
// usando AES-256-GCM autenticado. La master key se deriva via HKDF-SHA256 para
// obtener 32 bytes estables sin importar el formato del secreto configurado.
type Service struct {
	key [32]byte
}

// New crea el servicio a partir de la llave maestra en texto plano o base64.
// Acepta cualquier longitud >=16 y la deriva a 32 bytes con HKDF.
func New(masterKey string) (*Service, error) {
	trimmed := strings.TrimSpace(masterKey)
	if trimmed == "" {
		return nil, errors.New("crypto: master key vacía (configure ENCRYPTION_KEY / MASTER_ENCRYPTION_KEY)")
	}
	// Intentar base64 primero
	var raw []byte
	if decoded, err := base64.StdEncoding.DecodeString(trimmed); err == nil && len(decoded) >= 16 {
		raw = decoded
	} else if decoded, err := base64.RawStdEncoding.DecodeString(trimmed); err == nil && len(decoded) >= 16 {
		raw = decoded
	} else {
		raw = []byte(trimmed)
	}
	if len(raw) < 16 {
		return nil, errors.New("crypto: master key debe tener al menos 16 bytes")
	}
	// Derivar 32 bytes estables via HKDF
	h := hkdf.New(sha256.New, raw, nil, []byte("supay-siat-encryption-v1"))
	var key [32]byte
	if _, err := io.ReadFull(h, key[:]); err != nil {
		return nil, fmt.Errorf("crypto: no se pudo derivar la clave: %w", err)
	}
	return &Service{key: key}, nil
}

// Encrypt cifra plaintext con AES-GCM y retorna base64(nonce+ciphertext).
func (s *Service) Encrypt(plaintext []byte) (string, error) {
	if len(plaintext) == 0 {
		return "", errors.New("crypto: plaintext vacío")
	}
	block, err := aes.NewCipher(s.key[:])
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ct := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ct), nil
}

// Decrypt descifra base64(nonce+ciphertext) producido por Encrypt.
func (s *Service) Decrypt(ciphertextB64 string) ([]byte, error) {
	trimmed := strings.TrimSpace(ciphertextB64)
	if trimmed == "" {
		return nil, errors.New("crypto: ciphertext vacío")
	}
	raw, err := base64.StdEncoding.DecodeString(trimmed)
	if err != nil {
		// intentar RawStd
		raw, err = base64.RawStdEncoding.DecodeString(trimmed)
		if err != nil {
			return nil, fmt.Errorf("crypto: base64 inválido: %w", err)
		}
	}
	block, err := aes.NewCipher(s.key[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(raw) < gcm.NonceSize() {
		return nil, errors.New("crypto: ciphertext demasiado corto")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	pt, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: descifrado falló (clave incorrecta o datos corruptos): %w", err)
	}
	return pt, nil
}

// EncryptString helper para strings.
func (s *Service) EncryptString(plain string) (string, error) {
	return s.Encrypt([]byte(plain))
}

// DecryptString helper para strings.
func (s *Service) DecryptString(ciphertextB64 string) (string, error) {
	b, err := s.Decrypt(ciphertextB64)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// MustNew panics if key invalid - útil para tests.
func MustNew(masterKey string) *Service {
	s, err := New(masterKey)
	if err != nil {
		panic(err)
	}
	return s
}
