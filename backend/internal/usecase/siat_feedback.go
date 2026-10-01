package usecase

import (
	"log/slog"
	"sort"

	"github.com/brandsrx/supay/internal/ports"
)

// logSIATRejection emite el insumo mínimo del loop de feedback. Solo registra
// códigos de mensajes: las descripciones pueden contener valores reflejados del
// contribuyente y por tanto no son seguras para logs o issues automáticos.
func logSIATRejection(sector, codigoEstado int, messages []ports.FiscalMessage) {
	slog.Warn("rechazo SIAT listo para triage",
		"event", "siat_rejection_feedback",
		"sector", sector,
		"codigo_estado", codigoEstado,
		"mensajes", siatFeedbackMessageCodes(messages),
	)
}

func siatFeedbackMessageCodes(messages []ports.FiscalMessage) []int {
	seen := make(map[int]struct{}, len(messages))
	for _, message := range messages {
		seen[message.Codigo] = struct{}{}
	}
	codes := make([]int, 0, len(seen))
	for code := range seen {
		codes = append(codes, code)
	}
	sort.Ints(codes)
	return codes
}
