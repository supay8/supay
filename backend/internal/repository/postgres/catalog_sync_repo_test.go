package postgres

import (
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

func crearCompanyYPos(t *testing.T, db *gorm.DB) (string, string) {
	t.Helper()
	cid, pid := uuid.NewString(), uuid.NewString()
	if err := db.Create(&models.Company{ID: cid, Nit: uuid.NewString()[:20], BusinessName: "TEST", CodigoSistema: "SYS"}).Error; err != nil {
		t.Fatalf("company: %v", err)
	}
	if err := db.Create(&models.PointOfSale{ID: pid, CompanyId: cid, CodigoSucursal: 0, CodigoPuntoVenta: 1, Description: "PV"}).Error; err != nil {
		t.Fatalf("pos: %v", err)
	}
	return cid, pid
}

// TestUpsertSyncStateIdempotente verifica que re-sincronizar no viole la
// unicidad company+pos+operación del estado de sincronización.
func TestUpsertSyncStateIdempotente(t *testing.T) {
	db := newTestDB(t)
	cid, pid := crearCompanyYPos(t, db)
	repo := NewPostgresCatalogSyncStateRepository(db)
	now := time.Now()
	state := domain.CatalogSyncState{CompanyID: cid, PointOfSaleID: pid, Operation: "tipoMoneda", Status: "SUCCESS", RowsSaved: 10, SyncedAt: &now}
	if err := repo.Upsert(state); err != nil {
		t.Fatalf("primer upsert: %v", err)
	}
	state.Status = "STALE"
	state.RowsSaved = 11
	if err := repo.Upsert(state); err != nil {
		t.Fatalf("segundo upsert: %v", err)
	}
	states, err := repo.List(cid, pid)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(states) != 1 || states[0].Status != "STALE" || states[0].RowsSaved != 11 {
		t.Errorf("estado final inesperado: %+v", states)
	}
}

// TestSinProductReplaceDeduplica verifica que el catálogo de productos SIN
// tolere respuestas del SIAT con pares actividad-producto repetidos.
func TestSinProductReplaceDeduplica(t *testing.T) {
	db := newTestDB(t)
	cid, _ := crearCompanyYPos(t, db)
	repo := NewPostgresSinProductRepository(db)
	now := time.Now()
	productos := []domain.SinProduct{
		{CodigoProductoSin: 1004879, CodigoActividad: 8550100, Descripcion: "Curso A"},
		{CodigoProductoSin: 1004385, CodigoActividad: 8549100, Descripcion: "Capacitación"},
		{CodigoProductoSin: 1004879, CodigoActividad: 8550100, Descripcion: "Curso B"},
	}
	if err := repo.Replace(cid, productos, now); err != nil {
		t.Fatalf("replace con duplicados: %v", err)
	}
	items, total, err := repo.List(cid, "", 0, 50, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 2 || len(items) != 2 {
		t.Fatalf("se esperaban 2 productos deduplicados, hay %d/%d", len(items), total)
	}
	if items[0].CodigoActividad != 8549100 {
		t.Errorf("codigo_actividad=%d, se esperaba 8549100", items[0].CodigoActividad)
	}

	// Replace posterior reemplaza el catálogo completo.
	if err := repo.Replace(cid, productos[:1], now); err != nil {
		t.Fatalf("replace final: %v", err)
	}
	if _, total, _ = repo.List(cid, "", 0, 50, 0); total != 1 {
		t.Errorf("tras el replace debería haber 1 producto, hay %d", total)
	}
}

// TestSiatCatalogosDedicadosRoundTrip verifica los repos de actividades,
// leyendas y relación actividad-sector.
func TestSiatCatalogosDedicadosRoundTrip(t *testing.T) {
	db := newTestDB(t)
	cid, _ := crearCompanyYPos(t, db)
	now := time.Now()

	actRepo := NewPostgresSiatActividadRepository(db)
	if err := actRepo.Replace(cid, []domain.SiatActividad{
		{CodigoCaeb: "8549100", Descripcion: "Enseñanza secundaria", TipoActividad: "P"},
		{CodigoCaeb: "8550100", Descripcion: "Consultoría en educación", TipoActividad: "P"},
	}, now); err != nil {
		t.Fatalf("actividades replace: %v", err)
	}
	acts, err := actRepo.List(cid)
	if err != nil || len(acts) != 2 || acts[0].TipoActividad != "P" || acts[0].CodigoCaeb != "8549100" {
		t.Errorf("actividades round trip: %+v (err=%v)", acts, err)
	}

	leyRepo := NewPostgresSiatLeyendaRepository(db)
	if err := leyRepo.Replace(cid, []domain.SiatLeyenda{
		{CodigoActividad: "8549100", DescripcionLeyenda: "Ley N° 453: ..."},
	}, now); err != nil {
		t.Fatalf("leyendas replace: %v", err)
	}
	leyendas, err := leyRepo.ListByActividad(cid, "8549100")
	if err != nil || len(leyendas) != 1 || leyendas[0].DescripcionLeyenda == "" {
		t.Errorf("leyendas por actividad: %+v (err=%v)", leyendas, err)
	}

	secRepo := NewPostgresSiatActividadDocSectorRepository(db)
	if err := secRepo.Replace(cid, []domain.SiatActividadDocSector{
		{CodigoActividad: "8549100", CodigoDocumentoSector: 11, TipoDocumentoSector: "FSEDU"},
		{CodigoActividad: "8549100", CodigoDocumentoSector: 24, TipoDocumentoSector: "NCD"},
	}, now); err != nil {
		t.Fatalf("doc sector replace: %v", err)
	}
	sectores, err := secRepo.ListByActividad(cid, "8549100")
	if err != nil || len(sectores) != 2 || sectores[0].TipoDocumentoSector != "FSEDU" {
		t.Errorf("sectores por actividad: %+v (err=%v)", sectores, err)
	}
}
