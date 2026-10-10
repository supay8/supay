package usecase

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/brandsrx/supay/internal/domain/fiscal"
)

func TestParseFechaSiatLaPaz(t *testing.T) {
	got, err := ParseFechaSiat("2026-08-14T09:00:00.000")
	if err != nil {
		t.Fatalf("ParseFechaSiat: %v", err)
	}
	if got.In(fiscal.LaPaz).Hour() != 9 {
		t.Errorf("se esperaba 09:00 en America/La_Paz, got %v", got.In(fiscal.LaPaz))
	}
	if got.UTC().Hour() != 13 {
		t.Errorf("09:00 La Paz debe equivaler a 13:00Z, got %v", got.UTC())
	}
}

func TestParseFechaSiatEmptyUsesLaPaz(t *testing.T) {
	// El reloj comienza en 2000-01-01 00:00 UTC: en La Paz aún es 1999-12-31.
	synctest.Test(t, func(t *testing.T) {
		got, err := ParseFechaSiat("")
		if err != nil {
			t.Fatalf("ParseFechaSiat: %v", err)
		}
		if got.Location() != fiscal.LaPaz {
			t.Fatalf("la fecha vacía debe usar America/La_Paz, got %v", got.Location())
		}
		now := time.Now().In(fiscal.LaPaz)
		gotLaPaz := got.In(fiscal.LaPaz)
		gotYear, gotMonth, gotDay := gotLaPaz.Date()
		nowYear, nowMonth, nowDay := now.Date()
		if gotYear != nowYear || gotMonth != nowMonth || gotDay != nowDay {
			t.Errorf("la fecha vacía debe resolverse a hoy en America/La_Paz, got %v", gotLaPaz)
		}
	})
}

func TestParseFechaSiatInvalid(t *testing.T) {
	if _, err := ParseFechaSiat("2026-13-40T99:00:00.000"); err == nil {
		t.Fatal("se esperaba error para fecha inválida")
	}
}
