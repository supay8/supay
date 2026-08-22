package usecase

import (
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/siat"
)

func TestParseFechaSiatLaPaz(t *testing.T) {
	got, err := ParseFechaSiat("2026-08-14T09:00:00.000")
	if err != nil {
		t.Fatalf("ParseFechaSiat: %v", err)
	}
	if got.In(siat.LaPaz).Hour() != 9 {
		t.Errorf("se esperaba 09:00 en America/La_Paz, got %v", got.In(siat.LaPaz))
	}
	if got.UTC().Hour() != 13 {
		t.Errorf("09:00 La Paz debe equivaler a 13:00Z, got %v", got.UTC())
	}
}

func TestParseFechaSiatEmptyUsesLaPaz(t *testing.T) {
	got, err := ParseFechaSiat("")
	if err != nil {
		t.Fatalf("ParseFechaSiat: %v", err)
	}
	now := time.Now()
	if got.In(siat.LaPaz).Year() != now.Year() || got.In(siat.LaPaz).Day() != now.Day() {
		t.Errorf("la fecha vacía debe resolverse a hoy en America/La_Paz, got %v", got.In(siat.LaPaz))
	}
}

func TestParseFechaSiatInvalid(t *testing.T) {
	if _, err := ParseFechaSiat("2026-13-40T99:00:00.000"); err == nil {
		t.Fatal("se esperaba error para fecha inválida")
	}
}
