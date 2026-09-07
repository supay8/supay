package observability

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"strings"
)

// RedactingHandler removes credentials and PII from structured attributes.
// Identifiers are hashed so logs remain correlatable without exposing values.
type RedactingHandler struct {
	next slog.Handler
}

func NewRedactingHandler(next slog.Handler) slog.Handler {
	return &RedactingHandler{next: next}
}

func (h *RedactingHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.next.Enabled(ctx, level)
}

func (h *RedactingHandler) Handle(ctx context.Context, record slog.Record) error {
	clean := slog.NewRecord(record.Time, record.Level, record.Message, record.PC)
	record.Attrs(func(attr slog.Attr) bool {
		clean.AddAttrs(redactAttr(attr))
		return true
	})
	return h.next.Handle(ctx, clean)
}

func (h *RedactingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	clean := make([]slog.Attr, 0, len(attrs))
	for _, attr := range attrs {
		clean = append(clean, redactAttr(attr))
	}
	return &RedactingHandler{next: h.next.WithAttrs(clean)}
}

func (h *RedactingHandler) WithGroup(name string) slog.Handler {
	return &RedactingHandler{next: h.next.WithGroup(name)}
}

func redactAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()
	key := strings.ToLower(attr.Key)
	if attr.Value.Kind() == slog.KindGroup {
		group := attr.Value.Group()
		clean := make([]slog.Attr, 0, len(group))
		for _, child := range group {
			clean = append(clean, redactAttr(child))
		}
		return slog.Group(attr.Key, attrsToAny(clean)...)
	}
	if isSensitiveKey(key) {
		return slog.String(attr.Key, "[REDACTED]")
	}
	if key == "error" || strings.HasSuffix(key, "_error") {
		return slog.String(attr.Key, fmt.Sprintf("%T", attr.Value.Any()))
	}
	if isIdentifierKey(key) {
		return slog.String(attr.Key, hashValue(fmt.Sprint(attr.Value.Any())))
	}
	return attr
}

func attrsToAny(attrs []slog.Attr) []any {
	result := make([]any, 0, len(attrs))
	for _, attr := range attrs {
		result = append(result, attr)
	}
	return result
}

func isSensitiveKey(key string) bool {
	for _, part := range []string{
		"authorization", "cookie", "header", "api_key", "apikey", "token", "password", "secret",
		"email", "document_number", "documento", "complement", "telefono",
		"phone", "direccion", "address", "business_name", "nombre_cliente",
		"customer_name", "nit", "certificate", "cert_p12", "private_key",
		"path", "url", "endpoint", "ref", "cufd", "cuis", "codigo_cliente",
		"codigo_control", "codigo_recepcion",
	} {
		if strings.Contains(key, part) {
			return true
		}
	}
	return false
}

func isIdentifierKey(key string) bool {
	if key == "request_id" || key == "trace_id" {
		return false
	}
	return key == "id" || strings.HasSuffix(key, "_id") || strings.HasSuffix(key, "_ids")
}

func hashValue(value string) string {
	sum := sha256.Sum256([]byte(value))
	return fmt.Sprintf("sha256:%x", sum[:6])
}
