package app

import (
	"context"
	"testing"
	"time"

	appconfig "github.com/brandsrx/supay/internal/config"
)

type recordingMaintenanceRunner struct {
	calls chan string
}

func (r *recordingMaintenanceRunner) RunCredentialRenewal(context.Context) error {
	r.calls <- "credentials"
	return nil
}

func (r *recordingMaintenanceRunner) RunCertificateCheck(context.Context) error {
	r.calls <- "certificates"
	return nil
}

func TestMaintenanceSchedulerRunsBothJobsOnStartup(t *testing.T) {
	runner := &recordingMaintenanceRunner{calls: make(chan string, 2)}
	stop := StartMaintenanceScheduler(context.Background(), runner, appconfig.MaintenanceConfig{
		Enabled: true, CredentialInterval: time.Hour, CertificateInterval: 24 * time.Hour, JobTimeout: time.Second,
	})
	defer stop()

	for _, expected := range []string{"credentials", "certificates"} {
		select {
		case got := <-runner.calls:
			if got != expected {
				t.Fatalf("job=%q, se esperaba %q", got, expected)
			}
		case <-time.After(time.Second):
			t.Fatalf("el scheduler no ejecutó %s al arrancar", expected)
		}
	}
}
