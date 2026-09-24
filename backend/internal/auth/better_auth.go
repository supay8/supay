package auth

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const maxJWKSResponseBytes = 1 << 20

var errUnsupportedJWK = errors.New("tipo de JWK no soportado")

// BetterAuthVerifier valida los JWT de corta duración emitidos por el plugin
// JWT de Better Auth. Las llaves públicas se obtienen desde JWKS y se refrescan
// al vencer la caché o cuando aparece un kid nuevo durante una rotación.
type BetterAuthVerifier struct {
	jwksURL  string
	issuer   string
	audience string
	cacheTTL time.Duration
	client   *http.Client
	now      func() time.Time

	mu          sync.Mutex
	keys        map[string]verificationKey
	cacheUntil  time.Time
	unknownKids map[string]time.Time
}

type verificationKey struct {
	value any
	alg   string
}

type jwksDocument struct {
	Keys []json.RawMessage `json:"keys"`
}

type jsonWebKey struct {
	Kty string `json:"kty"`
	Use string `json:"use,omitempty"`
	Kid string `json:"kid"`
	Alg string `json:"alg,omitempty"`
	Crv string `json:"crv,omitempty"`
	X   string `json:"x,omitempty"`
	Y   string `json:"y,omitempty"`
	N   string `json:"n,omitempty"`
	E   string `json:"e,omitempty"`
}

func NewBetterAuthVerifier(jwksURL, issuer, audience string, cacheTTL, httpTimeout time.Duration) *BetterAuthVerifier {
	client := &http.Client{
		Timeout: httpTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) == 0 {
				return nil
			}
			origin := via[0].URL
			if req.URL.Scheme != origin.Scheme || req.URL.Host != origin.Host {
				return errors.New("redirección JWKS fuera del origen configurado")
			}
			if len(via) >= 3 {
				return errors.New("demasiadas redirecciones JWKS")
			}
			return nil
		},
	}
	return &BetterAuthVerifier{
		jwksURL:     jwksURL,
		issuer:      issuer,
		audience:    audience,
		cacheTTL:    cacheTTL,
		client:      client,
		now:         time.Now,
		keys:        make(map[string]verificationKey),
		unknownKids: make(map[string]time.Time),
	}
}

// VerifyAccessToken implementa el contrato usado por los middleware HTTP. No
// acepta session cookies de Better Auth: el frontend debe obtener el token del
// plugin JWT y enviarlo como Authorization: Bearer <token>.
func (v *BetterAuthVerifier) VerifyAccessToken(raw string) (string, error) {
	if v == nil || v.client == nil {
		return "", errors.New("verificador Better Auth no configurado")
	}
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(
		raw,
		claims,
		v.keyForToken,
		jwt.WithValidMethods([]string{"EdDSA", "ES256", "RS256", "PS256"}),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)
	if err != nil || !token.Valid {
		if err == nil {
			err = errors.New("token inválido")
		}
		return "", fmt.Errorf("validar access token de Better Auth: %w", err)
	}
	if claims.Subject == "" {
		return "", errors.New("access token de Better Auth sin subject")
	}
	if err := uuid.Validate(claims.Subject); err != nil {
		return "", errors.New("subject de Better Auth no es UUID")
	}
	return claims.Subject, nil
}

func (v *BetterAuthVerifier) keyForToken(token *jwt.Token) (any, error) {
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, errors.New("JWT sin kid")
	}
	alg := token.Method.Alg()

	v.mu.Lock()
	defer v.mu.Unlock()

	now := v.now().UTC()
	if key, found := v.keys[kid]; found && now.Before(v.cacheUntil) {
		if !keySupportsAlgorithm(key, alg) {
			return nil, fmt.Errorf("algoritmo %s incompatible con kid %s", alg, kid)
		}
		return key.value, nil
	}
	if retryAfter, found := v.unknownKids[kid]; found && now.Before(retryAfter) && now.Before(v.cacheUntil) {
		return nil, fmt.Errorf("kid %s desconocido", kid)
	}

	if err := v.refreshLocked(now); err != nil {
		return nil, err
	}
	key, found := v.keys[kid]
	if !found {
		negativeTTL := time.Minute
		if v.cacheTTL < negativeTTL {
			negativeTTL = v.cacheTTL
		}
		v.unknownKids[kid] = now.Add(negativeTTL)
		return nil, fmt.Errorf("kid %s no existe en JWKS", kid)
	}
	if !keySupportsAlgorithm(key, alg) {
		return nil, fmt.Errorf("algoritmo %s incompatible con kid %s", alg, kid)
	}
	return key.value, nil
}

func (v *BetterAuthVerifier) refreshLocked(now time.Time) error {
	req, err := http.NewRequest(http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return fmt.Errorf("crear petición JWKS: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := v.client.Do(req)
	if err != nil {
		return fmt.Errorf("descargar JWKS de Better Auth: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("descargar JWKS de Better Auth: HTTP %d", resp.StatusCode)
	}

	var document jwksDocument
	decoder := json.NewDecoder(io.LimitReader(resp.Body, maxJWKSResponseBytes))
	if err := decoder.Decode(&document); err != nil {
		return fmt.Errorf("decodificar JWKS de Better Auth: %w", err)
	}
	keys := make(map[string]verificationKey, len(document.Keys))
	for _, rawKey := range document.Keys {
		key, err := parseJWK(rawKey)
		if errors.Is(err, errUnsupportedJWK) {
			continue
		}
		if err != nil {
			return fmt.Errorf("decodificar llave JWKS: %w", err)
		}
		var metadata jsonWebKey
		if err := json.Unmarshal(rawKey, &metadata); err != nil {
			return fmt.Errorf("decodificar metadatos JWKS: %w", err)
		}
		if _, duplicate := keys[metadata.Kid]; duplicate {
			return fmt.Errorf("JWKS contiene kid duplicado %s", metadata.Kid)
		}
		keys[metadata.Kid] = verificationKey{value: key, alg: metadata.Alg}
	}
	if len(keys) == 0 {
		return errors.New("JWKS de Better Auth no contiene llaves de firma compatibles")
	}

	v.keys = keys
	v.cacheUntil = now.Add(v.cacheTTL)
	v.unknownKids = make(map[string]time.Time)
	return nil
}

func parseJWK(raw json.RawMessage) (any, error) {
	var key jsonWebKey
	if err := json.Unmarshal(raw, &key); err != nil {
		return nil, err
	}
	if key.Kid == "" {
		return nil, errors.New("JWK sin kid")
	}
	if key.Use != "" && key.Use != "sig" {
		return nil, errUnsupportedJWK
	}

	switch key.Kty {
	case "OKP":
		if key.Crv != "Ed25519" {
			return nil, errUnsupportedJWK
		}
		x, err := decodeBase64URL(key.X)
		if err != nil || len(x) != ed25519.PublicKeySize {
			return nil, errors.New("llave Ed25519 inválida")
		}
		return ed25519.PublicKey(x), nil
	case "EC":
		if key.Crv != "P-256" {
			return nil, errUnsupportedJWK
		}
		xBytes, err := decodeBase64URL(key.X)
		if err != nil {
			return nil, errors.New("coordenada x de EC inválida")
		}
		yBytes, err := decodeBase64URL(key.Y)
		if err != nil {
			return nil, errors.New("coordenada y de EC inválida")
		}
		x := new(big.Int).SetBytes(xBytes)
		y := new(big.Int).SetBytes(yBytes)
		curve := elliptic.P256()
		if !curve.IsOnCurve(x, y) {
			return nil, errors.New("llave EC fuera de la curva P-256")
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	case "RSA":
		nBytes, err := decodeBase64URL(key.N)
		if err != nil || len(nBytes) == 0 {
			return nil, errors.New("módulo RSA inválido")
		}
		eBytes, err := decodeBase64URL(key.E)
		if err != nil || len(eBytes) == 0 || len(eBytes) > 4 {
			return nil, errors.New("exponente RSA inválido")
		}
		e := 0
		for _, value := range eBytes {
			e = e<<8 | int(value)
		}
		if e < 3 || e%2 == 0 {
			return nil, errors.New("exponente RSA inválido")
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}, nil
	default:
		return nil, errUnsupportedJWK
	}
}

func decodeBase64URL(value string) ([]byte, error) {
	if value == "" {
		return nil, errors.New("valor base64url vacío")
	}
	return base64.RawURLEncoding.DecodeString(value)
}

func keySupportsAlgorithm(key verificationKey, alg string) bool {
	if key.alg != "" && key.alg != alg {
		return false
	}
	switch key.value.(type) {
	case ed25519.PublicKey:
		return alg == "EdDSA"
	case *ecdsa.PublicKey:
		return alg == "ES256"
	case *rsa.PublicKey:
		return alg == "RS256" || alg == "PS256"
	default:
		return false
	}
}
