package siat

import (
	"strings"
	"time"
)

// VentanaContingenciaHolgada calcula una ventana de contingencia amplia y
// holgada para evitar el error 1040 (fuera de rango) por desfase de relojes
// entre el VPS y los servidores del SIAT (piloto). Margen hacia atrás de
// 10 minutos es vital por latencias y desincronización.
// Retorna inicio = now -10m, fin = inicio +2h (= now +1h50m), ambos en LaPaz.
func VentanaContingenciaHolgada(now time.Time) (time.Time, time.Time) {
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(LaPaz)
	inicio := now.Add(-10 * time.Minute)
	fin := inicio.Add(2 * time.Hour)
	return inicio, fin
}

// DebeUsarVentanaHolgada determina si las fechas solicitadas requieren
// reemplazo por la ventana holgada. SOLO se usa si faltan fechas, falló el
// parse o fin <= inicio. Duración corta (ej. 11 min) y now fuera del rango
// son válidas SIAT (eventos históricos) y NO deben gatillar reemplazo.
func DebeUsarVentanaHolgada(inicio, fin time.Time, errInicio, errFin error) bool {
	if errInicio != nil || errFin != nil {
		return true
	}
	if inicio.IsZero() || fin.IsZero() {
		return true
	}
	if !fin.After(inicio) {
		return true
	}
	return false
}

// LaPaz es la zona horaria de Bolivia (UTC-4). El SIAT expresa la vigencia del
// CUFD y las fechas de los eventos significativos en esta hora local; no deben
// convertirse a UTC +00 al comparar o serializar.
var LaPaz = func() *time.Location {
	loc, err := time.LoadLocation("America/La_Paz")
	if err != nil {
		loc = time.FixedZone("BOT", -4*3600)
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

// FormatDisplayLaPaz convierte un timestamp a formato legible en zona horaria
// de Bolivia (UTC-4) para logs y respuestas HTTP. Si el timestamp es cero,
// retorna una cadena vacía.
func FormatDisplayLaPaz(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.In(LaPaz).Format("2006-01-02 15:04:05")
}
