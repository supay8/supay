package usecase

import (
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain/fiscal"
)

// ParseFechaSiat parsea una fecha/hora del SIAT en formato UTC extendido sin
// zona horaria (YYYY-MM-DDTHH:mm:ss.SSS). Si el valor está vacío usa la fecha
// y hora actual en Bolivia.
func ParseFechaSiat(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().In(fiscal.LaPaz), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02T15:04:05.000", value, fiscal.LaPaz)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}
