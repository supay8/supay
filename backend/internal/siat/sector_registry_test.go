package siat

import (
	"strings"
	"testing"

	goSiat "github.com/ron86i/go-siat/v2"
)

func TestSectorRegistryExponeVariantesYReglas(t *testing.T) {
	if got := len(PerfilesSector()); got != 52 {
		t.Fatalf("PerfilesSector()=%d, se esperaban 52 entradas: 51 casos SDK más sector 33 sin builder", got)
	}

	if _, err := PerfilSector(24); err == nil {
		t.Fatal("sector 24 debe exigir layout explícito")
	}
	standard24, err := PerfilSectorLayout(24, string(LayoutNotaCreditoDebito))
	if err != nil {
		t.Fatal(err)
	}
	fiscal24, err := PerfilSectorLayout(24, "nota_fiscal_credito_debito")
	if err != nil {
		t.Fatal(err)
	}
	if standard24.Layout == fiscal24.Layout || standard24.HasBuilder() != fiscal24.HasBuilder() {
		t.Fatalf("sector 24 no conserva sus dos layouts: %#v / %#v", standard24, fiscal24)
	}

	sector33, err := PerfilSector(33)
	if err != nil {
		t.Fatal(err)
	}
	if sector33.HasBuilder() {
		t.Fatal("sector 33 no debe declarar builders")
	}

	sector52, err := PerfilSector(52)
	if err != nil {
		t.Fatal(err)
	}
	if err := sector52.ValidarModalidad(ModalidadComputarizada); err == nil {
		t.Fatal("sector 52 debe rechazar modalidad computarizada")
	}
	if err := sector52.ValidarModalidad(ModalidadElectronica); err != nil {
		t.Fatalf("sector 52 electrónico: %v", err)
	}

	fixed, err := PerfilSector(1)
	if err != nil {
		t.Fatal(err)
	}
	if fixed.Facade.String() != "compra_venta" || fixed.Facade.IsByModalidad() {
		t.Fatalf("sector 1 debe usar fachada fija: %s", fixed.Facade)
	}
	modal, err := PerfilSector(2)
	if err != nil {
		t.Fatal(err)
	}
	if !modal.Facade.IsByModalidad() || modal.Facade.String() != "por_modalidad" {
		t.Fatalf("sector 2 debe usar fachada por modalidad: %s", modal.Facade)
	}
}

func TestSector33RequiereArchivoSinBuilder(t *testing.T) {
	base := SolicitudFactura{
		CodigoAmbiente:        AmbientePruebas,
		CodigoSistema:         "SYS",
		Nit:                   "1020304050",
		Modalidad:             ModalidadElectronica,
		CodigoDocumentoSector: 33,
	}
	if err := base.validate(); err == nil || !strings.Contains(err.Error(), "archivo y hashArchivo") {
		t.Fatalf("se esperaba validación de archivo/hash: %v", err)
	}

	base.Archivo = "BASE64"
	base.HashArchivo = "HASH"
	if err := base.validate(); err != nil {
		t.Fatalf("sector 33 con archivo/hash: %v", err)
	}

	base.CodigoAmbiente = 0
	base.CodigoSistema = ""
	base.Nit = ""
	if err := base.validate(); err != nil {
		t.Fatalf("la validación de bajo nivel no debe exigir identidad antes de Config: %v", err)
	}
}

func TestIdentidadUsaConfigYRechazaDiferencias(t *testing.T) {
	cfg := goSiatConfigForTest()
	ambiente := 0
	sistema := ""
	nit := ""
	if err := applyIdentityValues(cfg, &ambiente, &sistema, &nit); err != nil {
		t.Fatal(err)
	}
	if ambiente != cfg.CodigoAmbiente || sistema != cfg.CodigoSistema || nit != "1020304050" {
		t.Fatalf("identidad heredada incorrectamente: %d/%s/%s", ambiente, sistema, nit)
	}

	ambiente = cfg.CodigoAmbiente
	sistema = "OTRO"
	nit = "1020304050"
	if err := applyIdentityValues(cfg, &ambiente, &sistema, &nit); err == nil {
		t.Fatal("se esperaba rechazo de identidad diferente")
	}
}

func goSiatConfigForTest() goSiat.Config {
	return goSiat.Config{Nit: 1020304050, CodigoSistema: "SYS", CodigoAmbiente: AmbientePruebas}
}
