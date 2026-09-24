package config

import (
	"strings"
	"testing"
	"time"
)

func TestValidateAuthSelfHostedRequiresJWTSecret(t *testing.T) {
	cfg := Config{DeploymentMode: "selfhosted", JWTSecret: "short"}
	if err := cfg.ValidateAuth(); err == nil || !strings.Contains(err.Error(), "JWT_SECRET") {
		t.Fatalf("se esperaba error de JWT_SECRET, se obtuvo %v", err)
	}

	cfg.JWTSecret = strings.Repeat("x", 32)
	if err := cfg.ValidateAuth(); err != nil {
		t.Fatalf("config self-hosted válida rechazada: %v", err)
	}
}

func TestValidateAuthCloudUsesBetterAuthInsteadOfJWTSecret(t *testing.T) {
	cfg := Config{
		DeploymentMode: "cloud",
		BetterAuth: BetterAuthConfig{
			JWKSURL:      "https://auth.supay.example/api/auth/jwks",
			Issuer:       "https://auth.supay.example",
			Audience:     "https://auth.supay.example",
			JWKSCacheTTL: time.Hour,
			HTTPTimeout:  5 * time.Second,
		},
	}
	if err := cfg.ValidateAuth(); err != nil {
		t.Fatalf("config cloud válida rechazada: %v", err)
	}
}

func TestValidateAuthCloudRejectsInsecureRemoteJWKS(t *testing.T) {
	cfg := Config{
		DeploymentMode: "cloud",
		BetterAuth: BetterAuthConfig{
			JWKSURL:      "http://auth.example/api/auth/jwks",
			Issuer:       "http://auth.example",
			Audience:     "http://auth.example",
			JWKSCacheTTL: time.Hour,
			HTTPTimeout:  5 * time.Second,
		},
	}
	if err := cfg.ValidateAuth(); err == nil || !strings.Contains(err.Error(), "http solo") {
		t.Fatalf("se esperaba rechazo de JWKS remoto por HTTP, se obtuvo %v", err)
	}

	cfg.BetterAuth.JWKSURL = "http://127.0.0.1:3000/api/auth/jwks"
	if err := cfg.ValidateAuth(); err != nil {
		t.Fatalf("localhost HTTP debe permitirse para desarrollo: %v", err)
	}
}

func TestLoadDerivesBetterAuthContractFromBaseURL(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "cloud")
	t.Setenv("BETTER_AUTH_URL", "https://auth.supay.example/")
	t.Setenv("BETTER_AUTH_JWKS_URL", "")
	t.Setenv("BETTER_AUTH_ISSUER", "")
	t.Setenv("BETTER_AUTH_AUDIENCE", "")

	cfg := Load()
	if cfg.BetterAuth.URL != "https://auth.supay.example" {
		t.Fatalf("URL inesperada: %q", cfg.BetterAuth.URL)
	}
	if cfg.BetterAuth.JWKSURL != "https://auth.supay.example/api/auth/jwks" {
		t.Fatalf("JWKS URL inesperada: %q", cfg.BetterAuth.JWKSURL)
	}
	if cfg.BetterAuth.Issuer != cfg.BetterAuth.URL || cfg.BetterAuth.Audience != cfg.BetterAuth.URL {
		t.Fatalf("issuer/audience deben heredar BETTER_AUTH_URL: %#v", cfg.BetterAuth)
	}
}

func TestLoadParsesCORSAllowedOrigins(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGINS", "https://app.supay.bo, https://admin.supay.bo,https://app.supay.bo")

	cfg := Load()
	if len(cfg.CORSAllowedOrigins) != 2 || cfg.CORSAllowedOrigins[0] != "https://app.supay.bo" || cfg.CORSAllowedOrigins[1] != "https://admin.supay.bo" {
		t.Fatalf("orígenes CORS inesperados: %#v", cfg.CORSAllowedOrigins)
	}
}
