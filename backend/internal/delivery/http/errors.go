package http

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"reflect"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/usecase"
	"gorm.io/gorm"

	goSiat "github.com/ron86i/go-siat/v2"
)

// Taxonomía de códigos de error del contrato HTTP. Los clientes (y el SDK)
// programan contra estos códigos, nunca contra los mensajes: el texto puede
// cambiar, el código no.
const (
	CodeValidation      = "VALIDATION_ERROR"
	CodeNotFound        = "NOT_FOUND"
	CodeConflict        = "CONFLICT"
	CodeUnauthorized    = "UNAUTHORIZED"
	CodeSiatRejected    = "SIAT_REJECTED"
	CodeSiatUnavailable = "SIAT_UNAVAILABLE"
	CodeInternal        = "INTERNAL"
	CodeRateLimited     = "RATE_LIMITED"
)

// errorDetail es un mensaje individual dentro de un error (p.ej. cada
// observación devuelta por el SIAT en un rechazo).
type errorDetail struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// errorBody es el payload estándar de error.
type errorBody struct {
	Code        string        `json:"code"`
	Message     string        `json:"message"`
	Field       string        `json:"field,omitempty"`
	Suggestions []string      `json:"sugerencias,omitempty"`
	Action      string        `json:"accion,omitempty"`
	InvoiceID   string        `json:"invoice_id,omitempty"`
	Details     []errorDetail `json:"details,omitempty"`
}

// errorEnvelope es la forma única de error en toda la API:
// {"error": {"code": "...", "message": "...", "details": [...]}}.
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// classifyError mapea cualquier error de la aplicación a su status HTTP y
// cuerpo público. Es el ÚNICO punto de mapeo error→respuesta.
func classifyError(err error) (int, errorBody) {
	var rejected *usecase.EmissionRejectedError
	if errors.As(err, &rejected) {
		details := make([]errorDetail, 0, len(rejected.Mensajes))
		for _, m := range rejected.Mensajes {
			details = append(details, errorDetail{Code: m.Codigo, Message: m.Descripcion})
		}
		return http.StatusUnprocessableEntity, errorBody{
			Code:    CodeSiatRejected,
			Message: "el documento fue rechazado por el SIAT",
			Details: details,
		}
	}

	if errors.Is(err, usecase.ErrSiatNoDisponible) {
		return http.StatusServiceUnavailable, errorBody{
			Code:    CodeSiatUnavailable,
			Message: "el servicio SIAT no está disponible",
		}
	}

	var siatErr *goSiat.SiatError
	if errors.As(err, &siatErr) {
		if goSiat.IsNetworkError(err) || goSiat.IsRetryable(err) {
			return http.StatusServiceUnavailable, errorBody{
				Code:    CodeSiatUnavailable,
				Message: "el servicio SIAT no está disponible",
			}
		}
		return http.StatusBadGateway, errorBody{
			Code:    CodeSiatUnavailable,
			Message: "el servicio SIAT no respondió correctamente",
		}
	}

	var badRequest *domain.BadRequestError
	var notFound *domain.NotFoundError
	switch {
	case isConflict(err):
		return http.StatusConflict, errorBody{Code: CodeConflict, Message: err.Error()}
	case errors.As(err, &badRequest):
		return http.StatusBadRequest, errorBody{Code: CodeValidation, Message: err.Error()}
	case errors.As(err, &notFound), errors.Is(err, gorm.ErrRecordNotFound):
		return http.StatusNotFound, errorBody{Code: CodeNotFound, Message: err.Error()}
	default:
		// Error no tipado: se trata como interno. El detalle nunca viaja al
		// cliente (puede contener SQL, URLs de SOAP, etc.), solo se loguea.
		return http.StatusInternalServerError, errorBody{
			Code:    CodeInternal,
			Message: "error interno del servidor",
		}
	}
}

// enrichError convierte un error técnico en una respuesta que indica al
// integrador qué revisar y cuál es el siguiente paso seguro.
func enrichError(status int, body errorBody) errorBody {
	if body.Field == "" {
		body.Field = inferErrorField(body.Message)
	}
	if len(body.Suggestions) > 0 && body.Action != "" {
		return body
	}
	switch body.Code {
	case CodeValidation:
		body.Suggestions = []string{
			"Revise el tipo, formato y valor del campo indicado.",
			"Use POST /v1/invoices/preview para validar una factura antes de emitirla.",
		}
		body.Action = "Corrija el payload y vuelva a enviar la solicitud."
	case CodeUnauthorized:
		body.Suggestions = []string{"Envíe una credencial vigente en el header X-API-Key."}
		body.Action = "Corrija la autenticación y repita la solicitud."
	case CodeNotFound:
		body.Suggestions = []string{
			"Verifique que el identificador exista y pertenezca a la empresa autenticada.",
			"Consulte primero el recurso mediante su endpoint GET.",
		}
		body.Action = "Corrija el identificador y vuelva a intentar."
	case CodeConflict:
		body.Suggestions = []string{
			"Consulte el recurso existente antes de crear o modificar otro.",
			"Sincronice catálogos y parámetros SIAT si el conflicto depende de configuración tributaria.",
		}
		body.Action = "Resuelva el conflicto indicado y repita la operación."
	case CodeSiatRejected:
		body.Suggestions = []string{
			"Revise las observaciones incluidas en details.",
			"Valide nuevamente los datos y catálogos utilizados por la factura.",
		}
		body.Action = "Corrija la factura rechazada antes de solicitar una nueva emisión."
	case CodeSiatUnavailable:
		body.Suggestions = []string{
			"Espere unos segundos y consulte el estado de la factura.",
			"Mantenga la misma clave de idempotencia al reintentar la misma operación.",
		}
		body.Action = "Reintente más tarde; no duplique la factura."
	case CodeRateLimited:
		body.Suggestions = []string{"Respete el header Retry-After antes de realizar otro intento."}
		body.Action = "Espere el intervalo indicado y vuelva a intentar."
	default:
		if status >= http.StatusInternalServerError {
			body.Suggestions = []string{"Conserve el X-Request-ID de la respuesta para diagnóstico."}
			body.Action = "Reintente más tarde o contacte a soporte con el X-Request-ID."
		}
	}
	return body
}

func inferErrorField(message string) string {
	message = strings.ToLower(message)
	fields := []struct {
		field string
		terms []string
	}{
		{"Idempotency-Key", []string{"idempotency", "idempotencia"}},
		{"X-API-Key", []string{"x-api-key", "api key"}},
		{"point_of_sale_id", []string{"point_of_sale", "punto de venta"}},
		{"customer.document_number", []string{"document_number", "número de documento", "documento del cliente"}},
		{"customer.name", []string{"customer.name", "client_name", "nombre del cliente"}},
		{"reference_invoice_id", []string{"reference_invoice_id", "referencia_factura_id", "factura referenciada", "factura original"}},
		{"layout", []string{"layout"}},
		{"payment", []string{"payment"}},
		{"items[].data", []string{"datos_sector_detalle"}},
		{"data", []string{"datos_sector", "data."}},
		{"items[].sku", []string{"sku", "producto del ítem"}},
		{"items[].quantity", []string{"quantity", "cantidad"}},
		{"items[].price", []string{"price", "precio"}},
		{"items[].discount", []string{"discount", "descuento"}},
		{"invoice_type", []string{"invoice_type", "tipo de factura"}},
		{"sector", []string{"documento sector", "sector"}},
		{"nit", []string{"nit"}},
	}
	for _, candidate := range fields {
		for _, term := range candidate.terms {
			if strings.Contains(message, term) {
				return candidate.field
			}
		}
	}
	return ""
}

// isConflict reconoce los errores centinela de dominio (conflictos de
// unicidad y dependencias al eliminar) además de ConflictError tipado.
func isConflict(err error) bool {
	var conflict *domain.ConflictError
	if errors.As(err, &conflict) {
		return true
	}
	switch err {
	case domain.ErrCompanyNitConflict,
		domain.ErrCompanyHasDependencies,
		domain.ErrCustomerDocumentConflict,
		domain.ErrBranchSucursalConflict,
		domain.ErrPointOfSaleCodeConflict,
		domain.ErrPointOfSaleHasDependencies:
		return true
	}
	return false
}

// respondError es la salida uniforme de errores HTTP: clasifica el error,
// loguea el detalle completo server-side y escribe el envelope estándar.
func RespondError(w http.ResponseWriter, err error) {
	status, body := classifyError(err)
	if status >= 500 {
		slog.Error("request fallido", "status", status, "error_code", body.Code, "error_type", reflect.TypeOf(err), "error", err)
	} else {
		slog.Warn("request rechazado", "status", status, "error_code", body.Code, "error_type", reflect.TypeOf(err), "error", err)
	}
	writeErrorBody(w, status, body)
}

// respondErrorWithInvoiceID es igual a respondError pero anexa el invoice_id
// en el cuerpo cuando la operación ya creó/identificó una factura (p.ej. emisión
// rechazada por el SIAT). El invoiceID puede estar vacío; en ese caso delega a
// respondError sin modificar el envelope.
func RespondErrorWithInvoiceID(w http.ResponseWriter, err error, invoiceID string) {
	if invoiceID == "" {
		RespondError(w, err)
		return
	}
	status, body := classifyError(err)
	body.InvoiceID = invoiceID
	if status >= 500 {
		slog.Error("request fallido", "status", status, "invoice_id", invoiceID, "error_code", body.Code, "error_type", reflect.TypeOf(err))
	} else {
		slog.Warn("request rechazado", "status", status, "invoice_id", invoiceID, "error_code", body.Code, "error_type", reflect.TypeOf(err))
	}
	writeErrorBody(w, status, body)
}

// respondValidation responde 400 VALIDATION_ERROR con un mensaje público
// (payload ilegible, parámetro faltante, etc.).
func RespondValidation(w http.ResponseWriter, message string) {
	writeErrorBody(w, http.StatusBadRequest, errorBody{Code: CodeValidation, Message: message})
}

// respondNotFound responde 404 NOT_FOUND con un mensaje público.
func RespondNotFound(w http.ResponseWriter, message string) {
	writeErrorBody(w, http.StatusNotFound, errorBody{Code: CodeNotFound, Message: message})
}

// writeErrorBody serializa el envelope de error.
func writeErrorBody(w http.ResponseWriter, status int, body errorBody) {
	WriteJSON(w, status, errorEnvelope{Error: enrichError(status, body)})
}

// WriteErrorBody escribe un envelope de error con código y mensaje explícitos.
// Útil para módulos autónomos que necesitan devolver un error interno concreto
// sin pasar por la clasificación automática.
func WriteErrorBody(w http.ResponseWriter, status int, code, message string) {
	writeErrorBody(w, status, errorBody{Code: code, Message: message})
}

// listResponse es el envelope estándar de listados. limit/offset solo
// aparecen en los listados paginados.
type listResponse struct {
	Items  any `json:"items"`
	Total  int `json:"total"`
	Limit  int `json:"limit,omitempty"`
	Offset int `json:"offset,omitempty"`
}

// RespondList escribe un listado con envelope. Para listados sin paginación,
// pasar limit=offset=0 (los campos se omiten del JSON). Un slice nil se
// serializa como items: [] en lugar de null.
func RespondList(w http.ResponseWriter, items any, total, limit, offset int) {
	if items == nil {
		items = []any{}
	} else if rv := reflect.ValueOf(items); rv.Kind() == reflect.Slice && rv.IsNil() {
		items = reflect.MakeSlice(rv.Type(), 0, 0).Interface()
	}
	WriteJSON(w, http.StatusOK, listResponse{Items: items, Total: total, Limit: limit, Offset: offset})
}

// writeJSON serializa un payload con el status indicado.
func WriteJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
