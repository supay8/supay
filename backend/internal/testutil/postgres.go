// Package testutil contains infrastructure helpers shared by integration tests.
package testutil

import (
	"context"
	"database/sql"
	"testing"
	"time"
)

// LockPostgres serializes tests that migrate or truncate the same disposable
// database, including tests in different packages/processes under go test ./....
// A dedicated connection owns the session lock until all test cleanup finishes.
func LockPostgres(t testing.TB, db *sql.DB) {
	t.Helper()
	const lockID int64 = 0x7375706179 // "supay"; reserved for integration tests.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	conn, err := db.Conn(ctx)
	if err != nil {
		t.Fatalf("integration database lock connection: %v", err)
	}
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", lockID); err != nil {
		_ = conn.Close()
		t.Fatalf("integration database lock: %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_unlock($1)", lockID); err != nil {
			t.Errorf("release integration database lock: %v", err)
		}
		if err := conn.Close(); err != nil {
			t.Errorf("close integration database lock connection: %v", err)
		}
	})
}
