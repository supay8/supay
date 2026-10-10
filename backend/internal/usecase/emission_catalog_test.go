package usecase

import (
	"github.com/brandsrx/supay/internal/domain"
	"strings"
	"testing"
)

func TestCodigoTipoDocumentoIdentidad(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"CI", 1}, {"cex", 2}, {"PAS", 3}, {"NIT", 4}, {"OD", 5}, {"", -1}, {"XXX", -1},
	}
	for _, c := range cases {
		got, err := codigoTipoDocumentoIdentidad(c.in)
		if c.want == -1 {
			if err == nil {
				t.Errorf("codigoTipoDocumentoIdentidad(%q): se esperaba error", c.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("codigoTipoDocumentoIdentidad(%q): %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("codigoTipoDocumentoIdentidad(%q)=%d, se esperaba %d", c.in, got, c.want)
		}
	}
}

func TestResolveLeyenda(t *testing.T) {
	leyendas := &fakeLeyendaRepo{items: []*domain.SiatLeyenda{
		{CodigoActividad: "101010", DescripcionLeyenda: "Leyenda oficial de prueba"},
	}}
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	uc.leyendaRepo = leyendas

	got, err := uc.resolveLeyenda("comp-1", "101010")
	if err != nil {
		t.Fatalf("resolveLeyenda: %v", err)
	}
	if got != "Leyenda oficial de prueba" {
		t.Errorf("resolveLeyenda = %q, se esperaba la leyenda oficial", got)
	}

	// Sin catálogo sincronizado cae al fallback de la Ley 453.
	uc2 := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	got, err = uc2.resolveLeyenda("comp-1", "101010")
	if err != nil {
		t.Fatalf("resolveLeyenda fallback: %v", err)
	}
	if !strings.Contains(got, "453") {
		t.Errorf("fallback no contiene Ley 453: %q", got)
	}
}

func TestResolveDocumentoSector(t *testing.T) {
	sectores := &fakeDocSectorRepo{items: []*domain.SiatActividadDocSector{
		{CodigoActividad: "8549910", CodigoDocumentoSector: 1, TipoDocumentoSector: "FCV"},
		{CodigoActividad: "8550100", CodigoDocumentoSector: 1, TipoDocumentoSector: "FCV"},
		{CodigoActividad: "8549100", CodigoDocumentoSector: 11, TipoDocumentoSector: "FSEDU"},
		{CodigoActividad: "8549100", CodigoDocumentoSector: 24, TipoDocumentoSector: "NCD"},
		{CodigoActividad: "8549100", CodigoDocumentoSector: 47, TipoDocumentoSector: "NCDDE"},
	}}
	uc := newTestUsecase(newFakeInvoiceRepo(), &fakeCatalogRepo{}, nil)
	uc.docSectorRepo = sectores

	if got, err := uc.resolveDocumentoSector("comp-1", "8549100"); err != nil || got != 11 {
		t.Errorf("resolveDocumentoSector(8549100)=%d/%v, se esperaba 11 (FSEDU)", got, err)
	}
	if got, err := uc.resolveDocumentoSector("comp-1", "8549910"); err != nil || got != 1 {
		t.Errorf("resolveDocumentoSector(8549910)=%d/%v, se esperaba 1 (FCV)", got, err)
	}
	// Actividad sin asociación: no se permite un fallback estático.
	if _, err := uc.resolveDocumentoSector("comp-1", "9999999"); err == nil {
		t.Fatal("se esperaba error para actividad no sincronizada")
	}
}
