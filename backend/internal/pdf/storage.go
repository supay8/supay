// Package pdf — Storage abstraction for invoice PDFs.
package pdf

import (
	"context"
	"errors"
)

// ErrNotFound indica que el PDF no está en el storage (cache miss).
var ErrNotFound = errors.New("pdf storage: not found")

// Storage persiste y recupera PDFs de facturas. Implementaciones: noop, local, r2.
type Storage interface {
	// Save persiste el PDF para la factura indicada. No debe fallar la request si falla.
	Save(ctx context.Context, invoiceID string, data []byte) error
	// Get recupera el PDF. Retorna ErrNotFound si no existe.
	Get(ctx context.Context, invoiceID string) ([]byte, error)
	// Exists indica si el PDF está disponible sin leerlo.
	Exists(ctx context.Context, invoiceID string) bool
}
