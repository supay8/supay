package database

import "testing"

func TestPoolConfigUsesSmallCloudRunDefaults(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "cloud")
	t.Setenv("DB_MAX_OPEN", "")
	t.Setenv("DB_MAX_IDLE", "")
	got, err := poolConfigFromEnv()
	if err != nil {
		t.Fatalf("poolConfigFromEnv: %v", err)
	}
	if got.maxOpen != 10 || got.maxIdle != 5 {
		t.Fatalf("cloud pool=%+v, want MaxOpen=10 MaxIdle=5", got)
	}
}

func TestPoolConfigPreservesSelfHostedDefaults(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "selfhosted")
	t.Setenv("DB_MAX_OPEN", "")
	t.Setenv("DB_MAX_IDLE", "")
	got, err := poolConfigFromEnv()
	if err != nil {
		t.Fatalf("poolConfigFromEnv: %v", err)
	}
	if got.maxOpen != 60 || got.maxIdle != 15 {
		t.Fatalf("selfhosted pool=%+v, want MaxOpen=60 MaxIdle=15", got)
	}
}

func TestPoolConfigRejectsUnsafeCloudRunPool(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "cloud")
	t.Setenv("DB_MAX_OPEN", "60")
	t.Setenv("DB_MAX_IDLE", "5")
	if _, err := poolConfigFromEnv(); err == nil {
		t.Fatal("se esperaba rechazo de DB_MAX_OPEN inseguro para Cloud Run")
	}
}

func TestPoolConfigRejectsIdleAboveOpen(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "selfhosted")
	t.Setenv("DB_MAX_OPEN", "5")
	t.Setenv("DB_MAX_IDLE", "6")
	if _, err := poolConfigFromEnv(); err == nil {
		t.Fatal("se esperaba rechazo de DB_MAX_IDLE > DB_MAX_OPEN")
	}
}

func TestFactoryPoolModeDoesNotDependOnDeploymentEnvironment(t *testing.T) {
	t.Setenv("DEPLOYMENT_MODE", "selfhosted")
	t.Setenv("DB_MAX_OPEN", "")
	t.Setenv("DB_MAX_IDLE", "")
	got, err := poolConfigForMode("cloud")
	if err != nil || got.maxOpen != 10 || got.maxIdle != 5 {
		t.Fatalf("cloud factory pool=%+v err=%v", got, err)
	}
}
