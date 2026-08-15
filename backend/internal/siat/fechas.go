package siat

import (
	"strings"
	"time"
)

// LaPaz es la zona horaria de Bolivia (UTC-4). El SIAT expresa la vigencia del
// CUFD y las fechas de los eventos significativos en esta hora local; no deben
// convertirse a UTC +00 al comparar o serializar.
var LaPaz = func() *time.Location {
	loc, err := time.LoadLocation("America/La_Paz")
	if err != nil {
		loc = time.FixedZone("COT", -4*3600)
	}
	return loc
}()

// SanitizeCufd elimina de un CUFD todo carácter que no sea alfanumérico
// (espacios, saltos de línea, pipes, BOM, etc.) para evitar que el SIAT lo
// rechace o no lo reconozca (error 984).
func SanitizeCufd(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
			b.WriteRune(r)
		}
	}
	return b.String()
}

// SIATWallClockToInstant reinterpreta la hora de pared naive que devuelve el
// SIAT (p.ej. "2024-07-30T23:59:59.000", sin zona) como hora local de Bolivia
// para obtener el instante real. El SDK go-siat la parsea como UTC, por lo que
// sin este ajuste el valid_to almacenado queda 4 horas antes del instante real.
func SIATWallClockToInstant(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond(), LaPaz)
}
