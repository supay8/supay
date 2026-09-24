package auth

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type jwksFixture struct {
	mu     sync.RWMutex
	kid    string
	public ed25519.PublicKey
	calls  atomic.Int64
}

func (f *jwksFixture) handler(w http.ResponseWriter, _ *http.Request) {
	f.calls.Add(1)
	f.mu.RLock()
	defer f.mu.RUnlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"keys": []map[string]string{{
			"kty": "OKP",
			"use": "sig",
			"alg": "EdDSA",
			"crv": "Ed25519",
			"kid": f.kid,
			"x":   base64.RawURLEncoding.EncodeToString(f.public),
		}},
	})
}

func TestBetterAuthVerifierValidatesAndCachesJWKS(t *testing.T) {
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	fixture := &jwksFixture{kid: "key-1", public: public}
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	t.Cleanup(server.Close)

	verifier := NewBetterAuthVerifier(server.URL, "https://auth.supay.test", "supay-api", time.Hour, time.Second)
	userID := uuid.NewString()
	token := signBetterAuthToken(t, private, "key-1", userID, "https://auth.supay.test", "supay-api", time.Now().Add(time.Hour))

	for range 2 {
		actual, err := verifier.VerifyAccessToken(token)
		if err != nil {
			t.Fatalf("token válido rechazado: %v", err)
		}
		if actual != userID {
			t.Fatalf("subject inesperado: %s", actual)
		}
	}
	if calls := fixture.calls.Load(); calls != 1 {
		t.Fatalf("JWKS debía descargarse una sola vez, llamadas=%d", calls)
	}
}

func TestBetterAuthVerifierRefreshesUnknownKidAfterRotation(t *testing.T) {
	public1, private1, _ := ed25519.GenerateKey(rand.Reader)
	fixture := &jwksFixture{kid: "key-1", public: public1}
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	t.Cleanup(server.Close)
	verifier := NewBetterAuthVerifier(server.URL, "issuer", "audience", time.Hour, time.Second)

	first := signBetterAuthToken(t, private1, "key-1", uuid.NewString(), "issuer", "audience", time.Now().Add(time.Hour))
	if _, err := verifier.VerifyAccessToken(first); err != nil {
		t.Fatalf("primera llave rechazada: %v", err)
	}

	public2, private2, _ := ed25519.GenerateKey(rand.Reader)
	fixture.mu.Lock()
	fixture.kid = "key-2"
	fixture.public = public2
	fixture.mu.Unlock()
	second := signBetterAuthToken(t, private2, "key-2", uuid.NewString(), "issuer", "audience", time.Now().Add(time.Hour))
	if _, err := verifier.VerifyAccessToken(second); err != nil {
		t.Fatalf("llave rotada rechazada: %v", err)
	}
	if calls := fixture.calls.Load(); calls != 2 {
		t.Fatalf("se esperaba refresco por kid nuevo, llamadas=%d", calls)
	}
}

func TestBetterAuthVerifierRejectsClaimsOutsideContract(t *testing.T) {
	public, private, _ := ed25519.GenerateKey(rand.Reader)
	fixture := &jwksFixture{kid: "key-1", public: public}
	server := httptest.NewServer(http.HandlerFunc(fixture.handler))
	t.Cleanup(server.Close)
	verifier := NewBetterAuthVerifier(server.URL, "issuer", "audience", time.Hour, time.Second)

	tests := []struct {
		name     string
		subject  string
		issuer   string
		audience string
		expires  time.Time
	}{
		{name: "issuer", subject: uuid.NewString(), issuer: "foreign", audience: "audience", expires: time.Now().Add(time.Hour)},
		{name: "audience", subject: uuid.NewString(), issuer: "issuer", audience: "foreign", expires: time.Now().Add(time.Hour)},
		{name: "expired", subject: uuid.NewString(), issuer: "issuer", audience: "audience", expires: time.Now().Add(-time.Minute)},
		{name: "subject", subject: "not-a-uuid", issuer: "issuer", audience: "audience", expires: time.Now().Add(time.Hour)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			raw := signBetterAuthToken(t, private, "key-1", test.subject, test.issuer, test.audience, test.expires)
			if _, err := verifier.VerifyAccessToken(raw); err == nil {
				t.Fatal("se esperaba rechazo del token")
			}
		})
	}
}

func signBetterAuthToken(t *testing.T, private ed25519.PrivateKey, kid, subject, issuer, audience string, expires time.Time) string {
	t.Helper()
	now := time.Now().UTC()
	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   subject,
		Audience:  jwt.ClaimStrings{audience},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now.Add(-time.Second)),
		ExpiresAt: jwt.NewNumericDate(expires),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	token.Header["kid"] = kid
	raw, err := token.SignedString(private)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
