package config

import (
	"testing"
	"time"
)

func TestLoadRunMode(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want RunMode
	}{
		{name: "default", raw: "", want: RunModeBoth},
		{name: "web", raw: " WEB ", want: RunModeWeb},
		{name: "worker", raw: "worker", want: RunModeWorker},
		{name: "both", raw: "both", want: RunModeBoth},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("RUN_MODE", tt.raw)
			cfg := Load()
			if cfg.RunMode != tt.want {
				t.Fatalf("RunMode=%q, want %q", cfg.RunMode, tt.want)
			}
			if err := cfg.ValidateRunMode(); err != nil {
				t.Fatalf("ValidateRunMode: %v", err)
			}
		})
	}
}

func TestValidateRunModeRejectsUnknownValue(t *testing.T) {
	t.Setenv("RUN_MODE", "api-and-magic")
	cfg := Load()
	if err := cfg.ValidateRunMode(); err == nil {
		t.Fatal("se esperaba error para RUN_MODE desconocido")
	}
}

func TestRunModeComponentSelection(t *testing.T) {
	if !RunModeWeb.RunsWeb() || RunModeWeb.RunsWorker() {
		t.Fatal("web debe iniciar solo HTTP")
	}
	if RunModeWorker.RunsWeb() || !RunModeWorker.RunsWorker() {
		t.Fatal("worker debe iniciar solo procesos de fondo")
	}
	if !RunModeBoth.RunsWeb() || !RunModeBoth.RunsWorker() {
		t.Fatal("both debe iniciar HTTP y procesos de fondo")
	}
}

func TestEmissionSoftStopTimeoutNeverExceedsTenSeconds(t *testing.T) {
	for _, tt := range []struct {
		raw  string
		want time.Duration
	}{
		{raw: "", want: 10 * time.Second},
		{raw: "4s", want: 4 * time.Second},
		{raw: "30s", want: 10 * time.Second},
		{raw: "0s", want: 10 * time.Second},
	} {
		t.Run(tt.raw, func(t *testing.T) {
			t.Setenv("EMISSION_SOFT_STOP_TIMEOUT", tt.raw)
			if got := Load().Queue.SoftStopTimeout; got != tt.want {
				t.Fatalf("SoftStopTimeout=%s, want %s", got, tt.want)
			}
		})
	}
}

func TestOutboxLockTimeoutDefaultsBelowRetrySLO(t *testing.T) {
	t.Setenv("OUTBOX_LOCK_TIMEOUT", "")
	if got := Load().Queue.OutboxLockTimeout; got != 30*time.Second {
		t.Fatalf("OutboxLockTimeout=%s, want 30s", got)
	}
}
