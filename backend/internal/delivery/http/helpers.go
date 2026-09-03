package http

import "strconv"

// ParseQueryInt parsea un entero de un query param; si falla o es negativo
// devuelve el valor por defecto.
func ParseQueryInt(raw string, fallback int) int {
	if value, err := strconv.Atoi(raw); err == nil && value >= 0 {
		return value
	}
	return fallback
}
