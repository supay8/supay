package app

import (
	"strings"
	"testing"
	"time"

	authn "github.com/brandsrx/supay/internal/auth"
	"github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/repository/postgres"
)

func TestContainerSelectsSelfHostedAuthDependencies(t *testing.T) {
	container := NewContainer(config.Config{
		DeploymentMode: "selfhosted",
		JWTSecret:      strings.Repeat("s", 32),
		JWTIssuer:      "supay-test",
		JWTAccessTTL:   time.Hour,
	}, nil)

	if _, ok := container.AccessTokenVerifier().(*authn.JWTManager); !ok {
		t.Fatalf("self-hosted debe usar JWTManager, recibió %T", container.AccessTokenVerifier())
	}
	if _, ok := container.CompanyMemberships().(*postgres.PostgresAuthRepository); !ok {
		t.Fatalf("self-hosted debe usar PostgresAuthRepository, recibió %T", container.CompanyMemberships())
	}
}

func TestContainerSelectsBetterAuthCloudDependencies(t *testing.T) {
	container := NewContainer(config.Config{
		DeploymentMode: "cloud",
		BetterAuth: config.BetterAuthConfig{
			JWKSURL:      "https://auth.supay.test/api/auth/jwks",
			Issuer:       "https://auth.supay.test",
			Audience:     "supay-api",
			JWKSCacheTTL: time.Hour,
			HTTPTimeout:  time.Second,
		},
	}, nil)

	if _, ok := container.AccessTokenVerifier().(*authn.BetterAuthVerifier); !ok {
		t.Fatalf("cloud debe usar BetterAuthVerifier, recibió %T", container.AccessTokenVerifier())
	}
	if _, ok := container.CompanyMemberships().(*postgres.BetterAuthMembershipRepository); !ok {
		t.Fatalf("cloud debe usar BetterAuthMembershipRepository, recibió %T", container.CompanyMemberships())
	}
}
