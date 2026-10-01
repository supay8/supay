package usecase

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"github.com/brandsrx/supay/internal/ports"
)

func TestLogSIATRejectionNoIncluyeDescripcionPotencialmentePII(t *testing.T) {
	var output bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&output, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	logSIATRejection(1, 902, []ports.FiscalMessage{
		{Codigo: 1001, Descripcion: "NIT 1234567 rechazado para Ada Lovelace"},
		{Codigo: 1001, Descripcion: "duplicado"},
		{Codigo: 42, Descripcion: "otro"},
	})

	logLine := output.String()
	if strings.Contains(logLine, "1234567") || strings.Contains(logLine, "Ada Lovelace") {
		t.Fatalf("el feedback filtró PII: %s", logLine)
	}
	for _, expected := range []string{`"sector":1`, `"codigo_estado":902`, `"mensajes":[42,1001]`} {
		if !strings.Contains(logLine, expected) {
			t.Fatalf("falta %s en %s", expected, logLine)
		}
	}
}
