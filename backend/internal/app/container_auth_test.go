package app

import (
	"strings"
	"testing"
	"time"

	authn "github.com/brandsrx/supay/internal/auth"
	"github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/repository/postgres"
)

func TestFactorySelectsSelfHostedAuthDependencies(t *testing.T) {
	application := &App{cfg: config.Config{
		DeploymentMode: "selfhosted",
		JWTSecret:      strings.Repeat("s", 32),
		JWTIssuer:      "supay-test",
		JWTAccessTTL:   time.Hour,
	}}

	configureRepositories(application)
	bindings := selfHostedAuth(application)

	if _, ok := bindings.Tokens.(*authn.JWTManager); !ok {
		t.Fatalf("self-hosted debe usar JWTManager, recibió %T", bindings.Tokens)
	}
	if _, ok := bindings.Memberships.(*postgres.PostgresAuthRepository); !ok {
		t.Fatalf("self-hosted debe usar PostgresAuthRepository, recibió %T", bindings.Memberships)
	}
}

func TestFactorySelectsBetterAuthCloudDependencies(t *testing.T) {
	application := &App{cfg: config.Config{
		DeploymentMode: "cloud",
		BetterAuth: config.BetterAuthConfig{
			JWKSURL:      "https://auth.supay.test/api/auth/jwks",
			Issuer:       "https://auth.supay.test",
			Audience:     "supay-api",
			JWKSCacheTTL: time.Hour,
			HTTPTimeout:  time.Second,
		},
	}}

	bindings := cloudAuth(application)

	if _, ok := bindings.Tokens.(*authn.BetterAuthVerifier); !ok {
		t.Fatalf("cloud debe usar BetterAuthVerifier, recibió %T", bindings.Tokens)
	}
	if _, ok := bindings.Memberships.(*postgres.BetterAuthMembershipRepository); !ok {
		t.Fatalf("cloud debe usar BetterAuthMembershipRepository, recibió %T", bindings.Memberships)
	}
}
