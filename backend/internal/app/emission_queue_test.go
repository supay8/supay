package app

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fakeEmissionQueueLifecycle struct {
	startCtx   context.Context
	startCalls int
	stopCalls  int
	startErr   error
	stopErr    error
	stopBudget time.Duration
}

func (f *fakeEmissionQueueLifecycle) Start(ctx context.Context) error {
	f.startCtx = ctx
	f.startCalls++
	return f.startErr
}

func (f *fakeEmissionQueueLifecycle) Stop(ctx context.Context) error {
	f.stopCalls++
	if deadline, ok := ctx.Deadline(); ok {
		f.stopBudget = time.Until(deadline)
	}
	return f.stopErr
}

func TestEmissionQueueUsesGracefulStopBoundedToTenSeconds(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	queue := &fakeEmissionQueueLifecycle{}
	stop, err := StartEmissionQueue(parent, queue, true, 30*time.Second)
	if err != nil {
		t.Fatalf("StartEmissionQueue: %v", err)
	}
	cancel()
	select {
	case <-queue.startCtx.Done():
		t.Fatal("SIGTERM no debe cancelar River antes de ejecutar Stop")
	default:
	}
	if err := stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if queue.startCalls != 1 || queue.stopCalls != 1 {
		t.Fatalf("start=%d stop=%d", queue.startCalls, queue.stopCalls)
	}
	if queue.stopBudget <= 0 || queue.stopBudget > maxEmissionQueueStopTimeout {
		t.Fatalf("presupuesto de apagado=%s", queue.stopBudget)
	}
	if err := stop(); err != nil || queue.stopCalls != 1 {
		t.Fatalf("stop debe ser idempotente: calls=%d err=%v", queue.stopCalls, err)
	}
}

func TestEmissionQueuePropagatesLifecycleErrors(t *testing.T) {
	startErr := errors.New("river start")
	if stop, err := StartEmissionQueue(context.Background(), &fakeEmissionQueueLifecycle{startErr: startErr}, true, time.Second); !errors.Is(err, startErr) || stop != nil {
		t.Fatalf("start error: stop_nil=%t err=%v", stop == nil, err)
	}
	stopErr := errors.New("river stop")
	stop, err := StartEmissionQueue(context.Background(), &fakeEmissionQueueLifecycle{stopErr: stopErr}, true, time.Second)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := stop(); !errors.Is(err, stopErr) {
		t.Fatalf("stop error=%v", err)
	}
}

func TestEmissionQueueDisabledIsNoop(t *testing.T) {
	queue := &fakeEmissionQueueLifecycle{}
	stop, err := StartEmissionQueue(context.Background(), queue, false, time.Second)
	if err != nil || stop == nil {
		t.Fatalf("stop_nil=%t err=%v", stop == nil, err)
	}
	if err := stop(); err != nil || queue.startCalls != 0 || queue.stopCalls != 0 {
		t.Fatalf("disabled start=%d stop=%d err=%v", queue.startCalls, queue.stopCalls, err)
	}
}
