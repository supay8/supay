package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

type fakeStaleEmissionRepository struct {
	calls atomic.Int64
}

func (f *fakeStaleEmissionRepository) ReleaseStaleSending(time.Duration) (int64, error) {
	f.calls.Add(1)
	return 0, nil
}

func TestReaperStopsWithApplicationContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	repo := &fakeStaleEmissionRepository{}
	stop := startStaleEmissionReaper(ctx, repo, time.Millisecond, time.Minute)

	deadline := time.After(time.Second)
	for repo.calls.Load() == 0 {
		select {
		case <-deadline:
			t.Fatal("el reaper no ejecutó")
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	stop()
	time.Sleep(5 * time.Millisecond)
	afterCancel := repo.calls.Load()
	time.Sleep(5 * time.Millisecond)
	if got := repo.calls.Load(); got != afterCancel {
		t.Fatalf("el reaper siguió ejecutándose tras cancelar: before=%d after=%d", afterCancel, got)
	}
}
