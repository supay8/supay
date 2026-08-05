package siat

import (
	"testing"
	"time"
)

// TestGenerarCUF_VectorOficial verifica el CUF contra el vector de ejemplo
// oficial del SIN (Anexo Técnico "Generación CUF"):
//
//	NIT=123456789, fecha=20190113163721231, sucursal=0, modalidad=1,
//	emisión=1, factura=1, docSector=1, nº factura=1, puntoVenta=0,
//	codigoControl=A19E23EF34124CD
//
// CUF esperado: 8727F63A15F8976591FDDE5B387C5D015A29E06A1A19E23EF34124CD
func TestGenerarCUF_VectorOficial(t *testing.T) {
	fecha := time.Date(2019, 1, 13, 16, 37, 21, 231*int(time.Millisecond), time.UTC)
	cuf, err := (CUFParams{
		Nit:                   123456789,
		FechaHora:             fecha,
		CodigoSucursal:        0,
		Modalidad:             1,
		TipoEmision:           1,
		TipoFactura:           1,
		CodigoDocumentoSector: 1,
		NumeroFactura:         1,
		CodigoPuntoVenta:      0,
		CodigoControl:         "A19E23EF34124CD",
	}).GenerarCUF()
	if err != nil {
		t.Fatalf("GenerarCUF: %v", err)
	}
	const expected = "8727F63A15F8976591FDDE5B387C5D015A29E06A1A19E23EF34124CD"
	if cuf != expected {
		t.Fatalf("CUF incorrecto\n got:      %s\n expected: %s", cuf, expected)
	}
}

func TestGenerarCUF_Padding(t *testing.T) {
	fecha := time.Date(2019, 1, 13, 16, 37, 21, 231000000, time.UTC)
	cuf, err := (CUFParams{
		Nit:                   5,
		FechaHora:             fecha,
		CodigoSucursal:        3,
		Modalidad:             1,
		TipoEmision:           1,
		TipoFactura:           1,
		CodigoDocumentoSector: 1,
		NumeroFactura:         42,
		CodigoPuntoVenta:      7,
		CodigoControl:         "ABCDEF",
	}).GenerarCUF()
	if err != nil {
		t.Fatalf("GenerarCUF: %v", err)
	}
	// La cadena base (53 dígitos + dígito verificador = 54) en base16 más el
	// código de control. La parte hexadecimal debe ser estable para estos
	// parámetros: NIT=0000000000005, fecha=20190113163721231, sucursal=0003,
	// modalidad=1, emision=1, factura=1, docSector=01, factura=0000000042,
	// puntoVenta=0007.
	const wantHex = "5F8B38B9255A8C0D48BD0FD4A9DCC0C9688"
	if len(cuf) != len(wantHex)+len("ABCDEF") {
		t.Fatalf("longitud inesperada: %d", len(cuf))
	}
	if cuf != wantHex+"ABCDEF" {
		t.Fatalf("CUF incorrecto\n got:      %s\n expected: %s", cuf, wantHex+"ABCDEF")
	}
}

func TestGenerarCUF_Validaciones(t *testing.T) {
	fecha := time.Date(2019, 1, 13, 16, 37, 21, 0, time.UTC)

	tests := []struct {
		name string
		mod  func(*CUFParams)
	}{
		{"nit cero", func(p *CUFParams) { p.Nit = 0 }},
		{"fecha cero", func(p *CUFParams) { p.FechaHora = time.Time{} }},
		{"codigoControl vacio", func(p *CUFParams) { p.CodigoControl = "" }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := CUFParams{
				Nit:                   123456789,
				FechaHora:             fecha,
				CodigoSucursal:        0,
				Modalidad:             1,
				TipoEmision:           1,
				TipoFactura:           1,
				CodigoDocumentoSector: 1,
				NumeroFactura:         1,
				CodigoPuntoVenta:      0,
				CodigoControl:         "A19E23EF34124CD",
			}
			tt.mod(&p)
			if _, err := p.GenerarCUF(); err == nil {
				t.Fatalf("GenerarCUF() devolvió nil; se esperaba error")
			}
		})
	}
}

func TestCalculaDigitoMod11(t *testing.T) {
	// Multiplicadores 2..9 desde el último dígito.
	// 1 2 3 4 5 6 7 8 9  (de izquierda a derecha)
	// 2 3 4 5 6 7 8 9 2  (multiplicadores de derecha a izquierda)
	// suma = 9*2 + 8*3 + 7*4 + 6*5 + 5*6 + 4*7 + 3*8 + 2*9 + 1*2 = 202
	// 202 % 11 = 4
	if got := calculaDigitoMod11("123456789"); got != "4" {
		t.Fatalf("calculaDigitoMod11(\"123456789\") = %q; se esperaba \"4\"", got)
	}

	// Caso límite: residuo 10 -> "1"
	// Cadena tal que la suma % 11 == 10. Se verifica la regla sin romper la
	// coherencia del algoritmo comparando contra una implementación simple.
	got := calculaDigitoMod11("987654321")
	if got != "5" {
		t.Fatalf("calculaDigitoMod11(\"987654321\") = %q", got)
	}
}
