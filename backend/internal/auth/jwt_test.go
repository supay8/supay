package auth

import (
	"strings"
	"testing"
	"time"
)

func TestJWTManagerIssuesAndVerifiesAccessToken(t *testing.T) {
	manager := NewJWTManager(strings.Repeat("s", 48), "supay-test", time.Hour)
	token, expiresAt, err := manager.IssueAccessToken("user-123", "user@example.com")
	if err != nil {
		t.Fatalf("IssueAccessToken: %v", err)
	}
	if token == "" || time.Until(expiresAt) < 50*time.Minute {
		t.Fatalf("token o expiración inválidos: token=%t expires_at=%v", token != "", expiresAt)
	}
	userID, err := manager.VerifyAccessToken(token)
	if err != nil || userID != "user-123" {
		t.Fatalf("VerifyAccessToken userID=%q err=%v", userID, err)
	}
}

func TestJWTManagerRejectsTamperedAndForeignTokens(t *testing.T) {
	manager := NewJWTManager(strings.Repeat("a", 48), "supay-test", time.Hour)
	token, _, err := manager.IssueAccessToken("user-123", "user@example.com")
	if err != nil {
		t.Fatal(err)
	}
	tampered := token[:len(token)-1] + "x"
	if _, err := manager.VerifyAccessToken(tampered); err == nil {
		t.Fatal("se esperaba rechazo de token alterado")
	}
	other := NewJWTManager(strings.Repeat("b", 48), "supay-test", time.Hour)
	if _, err := other.VerifyAccessToken(token); err == nil {
		t.Fatal("se esperaba rechazo de token firmado con otro secreto")
	}
}

func TestJWTManagerRejectsWeakSecret(t *testing.T) {
	manager := NewJWTManager("short", "supay-test", time.Hour)
	if _, _, err := manager.IssueAccessToken("user-123", "user@example.com"); err == nil {
		t.Fatal("se esperaba error para JWT_SECRET débil")
	}
}
