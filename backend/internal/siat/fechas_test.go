package siat

import (
	"testing"
	"time"
)

func TestSanitizeCufd(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"ABC123", "ABC123"},
		{"ABC 123", "ABC123"},
		{" ABC\n123\r\t ", "ABC123"},
		{"a|b\u00a0c", "abc"},
		{"\uFEFFBOM\u0000CUFD", "BOMCUFD"},
		{"abc-123_def", "abc123def"},
	}
	for _, tc := range cases {
		if got := SanitizeCufd(tc.in); got != tc.want {
			t.Errorf("SanitizeCufd(%q) = %q, se esperaba %q", tc.in, got, tc.want)
		}
	}
}

func TestSIATWallClockToInstant(t *testing.T) {
	// El SDK parsea la hora de pared naive del SIAT como UTC (23:59:59Z).
	// El instante real es la misma hora de pared interpretada en America/La_Paz.
	naive := time.Date(2024, 7, 30, 23, 59, 59, 0, time.UTC)
	got := SIATWallClockToInstant(naive)

	want := time.Date(2024, 7, 31, 3, 59, 59, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("SIATWallClockToInstant = %v, se esperaba %v", got.UTC(), want.UTC())
	}
	if got.In(LaPaz).Format("2006-01-02T15:04:05") != "2024-07-30T23:59:59" {
		t.Errorf("la hora de pared de Bolivia debe conservarse, got %v", got.In(LaPaz))
	}
}

func TestLaPazOffset(t *testing.T) {
	if _, offset := time.Now().In(LaPaz).Zone(); offset != -4*3600 {
		t.Fatalf("America/La_Paz debe tener offset -4h, got %d", offset)
	}
}
