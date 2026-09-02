package usecase

import (
	"testing"

	"github.com/brandsrx/supay/internal/domain"
)

type stubCompanyRepo struct {
	company *domain.Company
}

func (r *stubCompanyRepo) Create(*domain.Company) error             { return nil }
func (r *stubCompanyRepo) GetByNit(string) (*domain.Company, error) { return r.company, nil }
func (r *stubCompanyRepo) GetByID(string) (*domain.Company, error)  { return r.company, nil }
func (r *stubCompanyRepo) Update(*domain.Company) error             { return nil }
func (r *stubCompanyRepo) Delete(string) error                      { return nil }

func TestResolveCatalogTipo(t *testing.T) {
	slug, tipo, ok := ResolveCatalogTipo("tipos-moneda")
	if !ok || slug != "tipos-moneda" || tipo != "tipoMoneda" {
		t.Fatalf("slug=%q tipo=%q ok=%v", slug, tipo, ok)
	}
	slug, tipo, ok = ResolveCatalogTipo("tipoMoneda")
	if !ok || slug != "tipos-moneda" || tipo != "tipoMoneda" {
		t.Fatalf("legado slug=%q tipo=%q ok=%v", slug, tipo, ok)
	}
	if _, _, ok = ResolveCatalogTipo("actividades"); ok {
		t.Fatal("actividades no es paramétrico genérico")
	}
}

func TestListParametricCatalog(t *testing.T) {
	parametricas := &fakeCatalogRepo{items: map[string][]*domain.CatalogItem{
		"tipoMoneda": {{Codigo: 1, Descripcion: "BOLIVIANO", Tipo: "tipoMoneda"}},
		"tipoMetodoPago": {
			{Codigo: 1, Descripcion: "Efectivo", Tipo: "tipoMetodoPago"},
			{Codigo: 2, Descripcion: "Tarjeta", Tipo: "tipoMetodoPago"},
		},
	}}
	uc := NewSiatUsecase(nil, nil, nil, nil, parametricas, nil, nil, nil, 0)

	res, err := uc.ListParametricCatalog("comp-1", "metodos-pago")
	if err != nil {
		t.Fatalf("ListParametricCatalog: %v", err)
	}
	if res.Catalog != "metodos-pago" || res.Total != 2 {
		t.Fatalf("res=%+v", res)
	}
	if _, err := uc.ListParametricCatalog("comp-1", "productos-sin"); err == nil {
		t.Fatal("productos-sin no debe resolverse como paramétrico")
	}
}

func TestListActividadesYDocumentosSector(t *testing.T) {
	actividades := &recordingActividadRepo{items: []domain.SiatActividad{
		{CodigoCaeb: "620100", Descripcion: "Software", TipoActividad: "P"},
		{CodigoCaeb: "474100", Descripcion: "Retail", TipoActividad: "S"},
	}}
	sectores := &recordingDocSectorRepo{items: []domain.SiatActividadDocSector{
		{CodigoActividad: "620100", CodigoDocumentoSector: 1, TipoDocumentoSector: "FCV"},
		{CodigoActividad: "474100", CodigoDocumentoSector: 1, TipoDocumentoSector: "FCV"},
	}}
	leyendas := &recordingLeyendaRepo{items: []domain.SiatLeyenda{
		{CodigoActividad: "620100", DescripcionLeyenda: "Leyenda A"},
		{CodigoActividad: "474100", DescripcionLeyenda: "Leyenda B"},
	}}
	principal := "620100"
	company := &stubCompanyRepo{company: &domain.Company{ID: "comp-1", CodigoActividad: &principal}}
	uc := NewSiatUsecase(company, nil, nil, nil, nil, nil, nil, nil, 0, actividades, sectores, leyendas)

	acts, err := uc.ListActividadesEconomicas("comp-1", "P")
	if err != nil || acts.Total != 1 || acts.Items[0].CodigoCaeb != "620100" {
		t.Fatalf("actividades filtradas: %+v err=%v", acts, err)
	}

	companyActs, err := uc.ListCompanyActividadesEconomicas("comp-1")
	if err != nil || companyActs.ActividadPrincipal == nil || companyActs.ActividadPrincipal.CodigoCaeb != "620100" {
		t.Fatalf("company acts: %+v err=%v", companyActs, err)
	}

	docs, err := uc.ListDocumentosSector("comp-1", "620100")
	if err != nil || docs.Total != 1 {
		t.Fatalf("docs: %+v err=%v", docs, err)
	}
	if docs.Items[0].Nombre == "" {
		t.Error("se esperaba nombre enriquecido del perfil Supay")
	}

	leys, err := uc.ListLeyendasFactura("comp-1", "620100")
	if err != nil || leys.Total != 1 {
		t.Fatalf("leyendas: %+v err=%v", leys, err)
	}

	boot, err := uc.EmisionBootstrap("comp-1", "")
	if err != nil {
		t.Fatalf("bootstrap: %v", err)
	}
	if len(boot.Actividades) != 2 || len(boot.DocumentosSector) != 1 || len(boot.Leyendas) != 1 {
		t.Fatalf("bootstrap incompleto: %+v", boot)
	}
}

func TestListProductosSinQuery(t *testing.T) {
	productos := &recordingSinProductRepo{items: []domain.SinProduct{
		{CodigoProductoSin: 1, CodigoActividad: 620100, Descripcion: "Servicio A"},
		{CodigoProductoSin: 2, CodigoActividad: 474100, Descripcion: "Producto B"},
	}}
	uc := NewSiatUsecase(nil, nil, nil, nil, nil, nil, nil, nil, 0, productos)

	res, err := uc.ListProductosSinQuery("comp-1", "", 620100, 50, 0)
	if err != nil || res.Total != 1 || res.Items[0].CodigoProductoSin != 1 {
		t.Fatalf("filtro actividad: %+v err=%v", res, err)
	}
}

func TestBuildSincronizacionResumen(t *testing.T) {
	uc := NewSiatUsecase(nil, nil, nil, nil, nil, nil, nil, nil, 0)
	res := &SincronizacionResultado{
		Company:     &domain.Company{ID: "c1"},
		PointOfSale: &domain.PointOfSale{ID: "p1"},
		Operations:  []SincronizacionOpResult{{Operation: "tipoMoneda", Status: "SUCCESS", RowsSaved: 2}},
	}
	out := uc.BuildSincronizacionResumen("c1", "p1", res)
	if !out.Success || out.CompanyID != "c1" || len(out.Operations) != 1 {
		t.Fatalf("resumen: %+v", out)
	}
}
