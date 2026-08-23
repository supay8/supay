package usecase

import (
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
)

type recordingActividadRepo struct {
	items []domain.SiatActividad
}

func (r *recordingActividadRepo) Replace(_ string, items []domain.SiatActividad, _ time.Time) error {
	r.items = items
	return nil
}

func (r *recordingActividadRepo) List(string) ([]*domain.SiatActividad, error) {
	out := make([]*domain.SiatActividad, 0, len(r.items))
	for i := range r.items {
		out = append(out, &r.items[i])
	}
	return out, nil
}

type recordingLeyendaRepo struct {
	items []domain.SiatLeyenda
}

func (r *recordingLeyendaRepo) Replace(_ string, items []domain.SiatLeyenda, _ time.Time) error {
	r.items = items
	return nil
}

func (r *recordingLeyendaRepo) List(string) ([]*domain.SiatLeyenda, error) {
	out := make([]*domain.SiatLeyenda, 0, len(r.items))
	for i := range r.items {
		out = append(out, &r.items[i])
	}
	return out, nil
}

func (r *recordingLeyendaRepo) ListByActividad(_ string, codigoActividad string) ([]*domain.SiatLeyenda, error) {
	out := make([]*domain.SiatLeyenda, 0)
	for i := range r.items {
		if r.items[i].CodigoActividad == codigoActividad {
			out = append(out, &r.items[i])
		}
	}
	return out, nil
}

type recordingDocSectorRepo struct {
	items []domain.SiatActividadDocSector
}

func (r *recordingDocSectorRepo) Replace(_ string, items []domain.SiatActividadDocSector, _ time.Time) error {
	r.items = items
	return nil
}

func (r *recordingDocSectorRepo) List(string) ([]*domain.SiatActividadDocSector, error) {
	out := make([]*domain.SiatActividadDocSector, 0, len(r.items))
	for i := range r.items {
		out = append(out, &r.items[i])
	}
	return out, nil
}

func (r *recordingDocSectorRepo) ListByActividad(_ string, codigoActividad string) ([]*domain.SiatActividadDocSector, error) {
	out := make([]*domain.SiatActividadDocSector, 0)
	for i := range r.items {
		if r.items[i].CodigoActividad == codigoActividad {
			out = append(out, &r.items[i])
		}
	}
	return out, nil
}

type recordingSinProductRepo struct {
	items []domain.SinProduct
}

func (r *recordingSinProductRepo) Replace(_ string, products []domain.SinProduct, _ time.Time) error {
	r.items = products
	return nil
}

func (r *recordingSinProductRepo) List(string, string, int, int) ([]*domain.SinProduct, int64, error) {
	out := make([]*domain.SinProduct, 0, len(r.items))
	for i := range r.items {
		out = append(out, &r.items[i])
	}
	return out, int64(len(r.items)), nil
}

func (r *recordingSinProductRepo) ListAll(string) ([]*domain.SinProduct, error) {
	out := make([]*domain.SinProduct, 0, len(r.items))
	for i := range r.items {
		out = append(out, &r.items[i])
	}
	return out, nil
}

func (r *recordingSinProductRepo) GetByCode(string, int64) (*domain.SinProduct, error) {
	for i := range r.items {
		if r.items[i].CodigoProductoSin == 1 {
			return &r.items[i], nil
		}
	}
	return nil, domain.ErrTipoPuntoVentaEmpty
}

func newPersistTestUsecase(actividades *recordingActividadRepo, leyendas *recordingLeyendaRepo, sectores *recordingDocSectorRepo, productos *recordingSinProductRepo) *SiatUsecase {
	return NewSiatUsecase(nil, nil, nil, nil, nil, nil, nil, nil, 0,
		productos, actividades, leyendas, sectores)
}

func TestPersistSincronizacionRutasDedicadas(t *testing.T) {
	actividades := &recordingActividadRepo{}
	leyendas := &recordingLeyendaRepo{}
	sectores := &recordingDocSectorRepo{}
	productos := &recordingSinProductRepo{}
	uc := newPersistTestUsecase(actividades, leyendas, sectores, productos)

	resAct := &siat.RespuestaSincronizacion{Transaccion: true, Actividades: []siat.ActividadDto{
		{CodigoCaeb: "8550100", Descripcion: "Consultoría en educación", TipoActividad: "P"},
	}}
	if err := uc.PersistSincronizacionAt("comp-1", "pos-1", siat.OpActividades, resAct); err != nil {
		t.Fatalf("persist actividades: %v", err)
	}
	if len(actividades.items) != 1 || actividades.items[0].TipoActividad != "P" || actividades.items[0].CodigoCaeb != "8550100" {
		t.Errorf("actividades mal persistidas: %+v", actividades.items)
	}

	resLey := &siat.RespuestaSincronizacion{Transaccion: true, Leyendas: []siat.LeyendaDto{
		{CodigoActividad: "8549910", DescripcionLeyenda: "Ley N° 453: Puedes acceder a la reclamación."},
	}}
	if err := uc.PersistSincronizacionAt("comp-1", "pos-1", siat.OpLeyendasFactura, resLey); err != nil {
		t.Fatalf("persist leyendas: %v", err)
	}
	if len(leyendas.items) != 1 || leyendas.items[0].CodigoActividad != "8549910" {
		t.Errorf("leyendas mal persistidas: %+v", leyendas.items)
	}

	resSec := &siat.RespuestaSincronizacion{Transaccion: true, ActividadesDocSector: []siat.ActividadDocSectorDto{
		{CodigoActividad: "8549100", CodigoDocumentoSector: 11, TipoDocumentoSector: "FSEDU"},
	}}
	if err := uc.PersistSincronizacionAt("comp-1", "pos-1", siat.OpActividadesDocumentoSector, resSec); err != nil {
		t.Fatalf("persist docSector: %v", err)
	}
	if len(sectores.items) != 1 || sectores.items[0].CodigoDocumentoSector != 11 || sectores.items[0].TipoDocumentoSector != "FSEDU" {
		t.Errorf("relaciones actividad-sector mal persistidas: %+v", sectores.items)
	}

	resProd := &siat.RespuestaSincronizacion{Transaccion: true, Productos: []siat.SinProductDto{
		{CodigoProductoSin: 1004385, CodigoActividad: 8549100, Descripcion: "Capacitación"},
	}}
	if err := uc.PersistSincronizacionAt("comp-1", "pos-1", siat.OpProductosServicios, resProd); err != nil {
		t.Fatalf("persist productos: %v", err)
	}
	if len(productos.items) != 1 || productos.items[0].CodigoActividad != 8549100 {
		t.Errorf("productos SIN mal persistidos: %+v", productos.items)
	}
}

// TestResolveDocumentoSectorDesdeTabla verifica que el documento-sector se
// resuelva desde la tabla dedicada con columnas reales.
func TestResolveDocumentoSectorDesdeTabla(t *testing.T) {
	sectores := &recordingDocSectorRepo{items: []domain.SiatActividadDocSector{
		{CodigoActividad: "8549100", CodigoDocumentoSector: 24, TipoDocumentoSector: "NCD"},
		{CodigoActividad: "8549100", CodigoDocumentoSector: 11, TipoDocumentoSector: "FSEDU"},
	}}
	uc := newPersistTestUsecase(nil, nil, sectores, nil)
	actividad := "8549100"
	company := &domain.Company{ID: "comp-1", CodigoActividad: &actividad}

	if got, err := uc.ResolveDocumentoSector(company); err != nil || got != 11 {
		t.Errorf("ResolveDocumentoSector=%d/%v, se esperaba 11 (FSEDU)", got, err)
	}
	sectores.items = append(sectores.items, domain.SiatActividadDocSector{CodigoActividad: "8549100", CodigoDocumentoSector: 1, TipoDocumentoSector: "FCV"})
	if got, err := uc.ResolveDocumentoSector(company); err != nil || got != 1 {
		t.Errorf("ResolveDocumentoSector=%d/%v, se esperaba 1 (FCV tiene prioridad)", got, err)
	}
}

// TestListCatalogTipos verifica la lectura unificada de catálogos.
func TestListCatalogTipos(t *testing.T) {
	actividades := &recordingActividadRepo{items: []domain.SiatActividad{
		{CodigoCaeb: "8550100", Descripcion: "Consultoría en educación", TipoActividad: "P"},
	}}
	parametricas := &fakeCatalogRepo{items: map[string][]*domain.CatalogItem{
		"tipoMoneda": {{Codigo: 1, Descripcion: "BOLIVIANO", Tipo: "tipoMoneda"}},
	}}
	uc := NewSiatUsecase(nil, nil, nil, nil, parametricas, nil, nil, nil, 0, actividades)

	res, err := uc.ListCatalog("comp-1", "actividades")
	if err != nil {
		t.Fatalf("ListCatalog actividades: %v", err)
	}
	out := res.(*CatalogoResultado)
	if out.Cantidad != 1 {
		t.Errorf("cantidad=%d, se esperaba 1", out.Cantidad)
	}
	act, ok := out.Items[0].(*domain.SiatActividad)
	if !ok || act.TipoActividad != "P" {
		t.Errorf("item de actividad inesperado: %+v", out.Items[0])
	}

	res, err = uc.ListCatalog("comp-1", "tipoMoneda")
	if err != nil {
		t.Fatalf("ListCatalog tipoMoneda: %v", err)
	}
	if out = res.(*CatalogoResultado); out.Cantidad != 1 {
		t.Errorf("paramétricas cantidad=%d, se esperaba 1", out.Cantidad)
	}

	if _, err = uc.ListCatalog("comp-1", "fechaHora"); err == nil {
		t.Error("fechaHora no almacena catálogo; se esperaba error")
	}
	if _, err = uc.ListCatalog("comp-1", "inexistente"); err == nil {
		t.Error("catálogo desconocido; se esperaba error")
	}
}
