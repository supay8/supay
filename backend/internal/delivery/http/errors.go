package http

import (
	"log/slog"
	"net/http"
)

// respondError es la salida uniforme de errores HTTP: envelope JSON
// {"error": "..."}, mapeo del error a status vía errToStatus y log del detalle
// completo server-side. En errores 5xx el mensaje interno no viaja al cliente
// para no filtrar detalles de SOAP/repositorio.
func respondError(w http.ResponseWriter, err error) {
	status := errToStatus(err)
	if status >= 500 {
		slog.Error("request fallido", "status", status, "error", err)
		writeJSONError(w, status, publicMessage(status))
		return
	}
	slog.Warn("request rechazado", "status", status, "error", err)
	writeJSONError(w, status, err.Error())
}

func publicMessage(status int) string {
	switch status {
	case http.StatusBadGateway:
		return "el servicio SIAT no respondió correctamente"
	case http.StatusServiceUnavailable:
		return "el servicio SIAT no está disponible"
	default:
		return "error interno del servidor"
	}
}
