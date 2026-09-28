package http

import "net/http"

// RequireTenant obtiene el tenant autenticado o responde 401. Los handlers de
// negocio deben llamarlo antes de usar identificadores controlados por el cliente.
func RequireTenant(w http.ResponseWriter, r *http.Request) (string, bool) {
	companyID, ok := CompanyIDFromContext(r.Context())
	if ok {
		return companyID, true
	}
	writeErrorBody(w, http.StatusUnauthorized, errorBody{
		Code:    CodeUnauthorized,
		Message: "no autorizado: falta el tenant autenticado",
	})
	return "", false
}
