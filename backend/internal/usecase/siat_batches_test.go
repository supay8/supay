package usecase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"gorm.io/gorm"
)

type batchTestRepository struct {
	domain.SentPackageRepository
	invoices              *fakeInvoiceRepo
	packages              map[string]domain.SentPackage
	reserves, updates     int
	reserveErr, updateErr error
}

func (r *batchTestRepository) GetByID(id string) (*domain.SentPackage, error) {
	p, ok := r.packages[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return &p, nil
}
func (r *batchTestRepository) ListPendingBatchInvoices(companyID, posID string, status domain.InvoiceStatus, eventID *string) ([]*domain.Invoice, error) {
	var out []*domain.Invoice
	for _, inv := range r.invoices.invoices {
		if inv.CompanyId != companyID || inv.PointOfSaleId != posID || inv.Status != status {
			continue
		}
		if eventID != nil && (inv.ContingencyEventId == nil || *inv.ContingencyEventId != *eventID) {
			continue
		}
		out = append(out, inv)
	}
	return out, nil
}
func (r *batchTestRepository) ReserveBatch(pkg *domain.SentPackage, ids []string, status domain.InvoiceStatus) error {
	r.reserves++
	if r.reserveErr != nil {
		return r.reserveErr
	}
	for _, id := range ids {
		if r.invoices.invoices[id].Status != status {
			return errors.New("factura reservada")
		}
	}
	if len(pkg.Documents) != len(ids) {
		return errors.New("documentos no preparados antes de reservar")
	}
	pkg.ID = fmt.Sprintf("batch-%d", r.reserves)
	pkg.SentAt = time.Now()
	for i, id := range ids {
		inv := r.invoices.invoices[id]
		inv.Status = domain.InvoiceSending
		doc := pkg.Documents[i]
		inv.Cuf = strPtr(doc.Cuf)
		inv.Xml = strPtr(doc.Xml)
	}
	r.packages[pkg.ID] = *pkg
	return nil
}
func (r *batchTestRepository) UpdateBatch(pkg *domain.SentPackage, status *domain.InvoiceStatus) error {
	r.updates++
	if r.updateErr != nil {
		return r.updateErr
	}
	r.packages[pkg.ID] = *pkg
	if status != nil {
		for _, id := range pkg.InvoiceIDs {
			r.invoices.invoices[id].Status = *status
		}
	}
	return nil
}

type batchTestService struct {
	ports.FiscalService
	prepareCalls, sendCalls, validateCalls int
	prepareErrAt                           int
	onSend                                 func([]ports.FiscalDocument) error
	onValidate                             func(ports.FiscalBulk, string)
	validateResult                         ports.FiscalPackageResult
	lastPackage                            ports.FiscalPackage
}

func (s *batchTestService) prepare(docs []ports.FiscalDocument) ([]ports.FiscalDocument, error) {
	s.prepareCalls++
	if s.prepareCalls == s.prepareErrAt {
		return nil, errors.New("XML inválido")
	}
	out := append([]ports.FiscalDocument(nil), docs...)
	for i := range out {
		if out[i].Cuf == "" {
			out[i].Cuf = fmt.Sprintf("CUF-%d", out[i].NumeroFactura)
		}
		if out[i].XML == "" {
			out[i].XML = fmt.Sprintf("<invoice>%d</invoice>", out[i].NumeroFactura)
		}
		out[i].Archivo = "gzip-documento"
		out[i].HashArchivo = "hash-documento"
	}
	return out, nil
}
func (s *batchTestService) PrepareBulk(_ context.Context, b ports.FiscalBulk) (ports.FiscalBulk, error) {
	var err error
	b.Facturas, err = s.prepare(b.Facturas)
	b.Archivo = "archivo-lote"
	b.HashArchivo = "hash-lote"
	return b, err
}
func (s *batchTestService) PreparePackage(_ context.Context, p ports.FiscalPackage) (ports.FiscalPackage, error) {
	var err error
	p.Facturas, err = s.prepare(p.Facturas)
	p.Archivo = "archivo-lote"
	p.HashArchivo = "hash-lote"
	return p, err
}
func (s *batchTestService) send(docs []ports.FiscalDocument) (ports.FiscalPackageResult, error) {
	s.sendCalls++
	if s.onSend != nil {
		if err := s.onSend(docs); err != nil {
			return ports.FiscalPackageResult{}, err
		}
	}
	return ports.FiscalPackageResult{Transaccion: true, CodigoEstado: 901, CodigoRecepcion: fmt.Sprintf("R-%d", s.sendCalls), CantidadFacturas: len(docs)}, nil
}
func (s *batchTestService) SendBulk(_ context.Context, b ports.FiscalBulk) (ports.FiscalPackageResult, error) {
	return s.send(b.Facturas)
}
func (s *batchTestService) SendPackage(_ context.Context, p ports.FiscalPackage) (ports.FiscalPackageResult, error) {
	s.lastPackage = p
	return s.send(p.Facturas)
}
func (s *batchTestService) ValidateBulk(_ context.Context, b ports.FiscalBulk, receipt string) (ports.FiscalPackageResult, error) {
	s.validateCalls++
	if s.onValidate != nil {
		s.onValidate(b, receipt)
	}
	return s.validateResult, nil
}
func (s *batchTestService) ValidatePackage(ctx context.Context, p ports.FiscalPackage, receipt string) (ports.FiscalPackageResult, error) {
	return s.ValidateBulk(ctx, ports.FiscalBulk{Cuis: p.Cuis, Cufd: p.Cufd, Modalidad: p.Modalidad, Layout: p.Layout, CodigoEmision: p.CodigoEmision, CodigoTipoFactura: p.CodigoTipoFactura, CodigoDocumentoSector: p.CodigoDocumentoSector, Facturas: p.Facturas}, receipt)
}

func batchFixture() (*SiatUsecase, *batchTestRepository, *batchTestService, *domain.Invoice) {
	inv := testInvoice()
	inv.Modalidad = siat.ModalidadElectronica
	inv.CodigoDocumentoSector = 1
	inv.CodigoTipoFactura = 1
	inv.CufdRecord.PointOfSaleID = inv.PointOfSaleId
	invoices := newFakeInvoiceRepo()
	invoices.invoices[inv.ID] = inv
	repo := &batchTestRepository{invoices: invoices, packages: map[string]domain.SentPackage{}}
	svc := &batchTestService{validateResult: ports.FiscalPackageResult{Transaccion: true, CodigoEstado: 908}}
	companyRepo := &fakeCompanyRepo{company: inv.Company}
	posRepo := &fakePointOfSaleRepo{pos: inv.PointOfSale}
	uc := &SiatUsecase{companyRepo: companyRepo, pointOfSaleRepo: posRepo, invoiceRepo: invoices, sentPackageRepo: repo, siatService: svc}
	uc.credentials = NewCredentialService(posRepo, &fakeCredCufdStore{existing: &inv.CufdRecord}, svc, siat.ModalidadElectronica)
	return uc, repo, svc, inv
}

func TestMasivaPreparaYGuardaDocumentosAntesDeEnviar(t *testing.T) {
	uc, repo, svc, inv := batchFixture()
	svc.onSend = func(docs []ports.FiscalDocument) error {
		if inv.Status != domain.InvoiceSending || inv.Cuf == nil || inv.Xml == nil || *inv.Xml != docs[0].XML {
			t.Fatal("envío sin reserva e identidad fiscal persistida")
		}
		if docs[0].Cufd != inv.CufdRecord.Cufd || docs[0].CodigoPuntoVenta != inv.PointOfSale.CodigoPuntoVenta {
			t.Fatal("identidad POS/CUFD incorrecta")
		}
		return nil
	}
	out, err := uc.EnviarMasiva(context.Background(), inv.CompanyId, inv.PointOfSaleId, MasivaInput{})
	if err != nil {
		t.Fatal(err)
	}
	if !out.Response.Transaccion || len(out.Batches) != 1 || out.Batches[0].BatchID == "" || inv.Status != domain.InvoiceSent {
		t.Fatalf("resultado inesperado: %+v", out)
	}
	stored := repo.packages[out.Batches[0].BatchID]
	if stored.CodigoEmision != 3 || stored.CodigoTipoFactura != 1 || len(stored.Documents) != 1 || stored.Documents[0].XmlHash == "" {
		t.Fatalf("snapshot incompleto: %+v", stored)
	}
}

func TestMasivaAgrupaYDivideSegunPerfilYLimite(t *testing.T) {
	uc, repo, svc, inv := batchFixture()
	for i := 2; i <= siat.MaxFacturasMasiva+2; i++ {
		copy := *inv
		copy.ID = fmt.Sprintf("inv-%04d", i)
		copy.InvoiceNumber = i
		repo.invoices.invoices[copy.ID] = &copy
	}
	separate := *inv
	separate.ID = "other-modality"
	separate.InvoiceNumber = siat.MaxFacturasMasiva + 3
	separate.Modalidad = siat.ModalidadComputarizada
	repo.invoices.invoices[separate.ID] = &separate
	out, err := uc.EnviarMasiva(context.Background(), inv.CompanyId, inv.PointOfSaleId, MasivaInput{})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Batches) != 3 || svc.prepareCalls != 3 || svc.sendCalls != 3 {
		t.Fatalf("lotes=%d preparaciones=%d envíos=%d", len(out.Batches), svc.prepareCalls, svc.sendCalls)
	}
	for _, batch := range out.Batches {
		if len(batch.InvoiceIDs) > siat.MaxFacturasMasiva {
			t.Fatal("límite excedido")
		}
	}
	if out.Response.CantidadFacturas != siat.MaxFacturasMasiva+3 {
		t.Fatalf("cantidad=%d", out.Response.CantidadFacturas)
	}
}

func TestMasivaErrorEnUltimoLoteNoReservaNiEnvia(t *testing.T) {
	uc, repo, svc, inv := batchFixture()
	other := *inv
	other.ID = "inv-2"
	other.InvoiceNumber = 2
	other.Modalidad = siat.ModalidadComputarizada
	repo.invoices.invoices[other.ID] = &other
	svc.prepareErrAt = 2
	_, err := uc.EnviarMasiva(context.Background(), inv.CompanyId, inv.PointOfSaleId, MasivaInput{})
	if err == nil || repo.reserves != 0 || svc.sendCalls != 0 || inv.Status != domain.InvoicePending {
		t.Fatalf("err=%v reservas=%d envíos=%d", err, repo.reserves, svc.sendCalls)
	}
}

func TestMasivaRechazaSeleccionNoAptaAntesDePreparar(t *testing.T) {
	tests := []struct {
		name   string
		change func(*domain.Invoice)
		ids    []string
	}{
		{name: "otra empresa", change: func(i *domain.Invoice) { i.CompanyId = "other" }},
		{name: "otro POS", change: func(i *domain.Invoice) { i.PointOfSaleId = "other" }},
		{name: "aceptada", change: func(i *domain.Invoice) { i.Status = domain.InvoiceAccepted }},
		{name: "identidad previa", change: func(i *domain.Invoice) { i.Cuf = strPtr("original") }},
		{name: "contingencia", change: func(i *domain.Invoice) { i.ContingencyEventId = strPtr("event") }},
		{name: "recepción previa", change: func(i *domain.Invoice) { i.SiatReceptionCode = strPtr("receipt") }},
		{name: "duplicada", ids: []string{"inv-1", "inv-1"}},
		{name: "inexistente", ids: []string{"missing"}},
		{name: "selección vacía", ids: []string{}},
		{name: "ID vacío", ids: []string{""}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc, repo, svc, inv := batchFixture()
			if tt.change != nil {
				tt.change(inv)
			}
			ids := tt.ids
			if ids == nil {
				ids = []string{inv.ID}
			}
			_, err := uc.EnviarMasiva(context.Background(), "comp-1", "pos-1", MasivaInput{FacturaIDs: ids})
			if err == nil || repo.reserves != 0 || svc.prepareCalls != 0 || svc.sendCalls != 0 {
				t.Fatalf("err=%v reserva=%d prepare=%d send=%d", err, repo.reserves, svc.prepareCalls, svc.sendCalls)
			}
		})
	}
}

func TestMasivaRespuestaInciertaConservaIdentidadYNoReenvia(t *testing.T) {
	uc, repo, svc, inv := batchFixture()
	svc.onSend = func([]ports.FiscalDocument) error { return errors.New("timeout tras enviar") }
	out, err := uc.EnviarMasiva(context.Background(), inv.CompanyId, inv.PointOfSaleId, MasivaInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Response.Transaccion || out.Batches[0].Status != domain.PackageStatusUnknown || inv.Cuf == nil || inv.Xml == nil || inv.Status != domain.InvoiceSending {
		t.Fatalf("resultado=%+v factura=%+v", out, inv)
	}
	if repo.packages[out.Batches[0].BatchID].Status != domain.PackageStatusUnknown {
		t.Fatal("no guardó resultado incierto")
	}
	again, err := uc.EnviarMasiva(context.Background(), inv.CompanyId, inv.PointOfSaleId, MasivaInput{})
	if err != nil || len(again.Batches) != 0 || svc.sendCalls != 1 {
		t.Fatalf("reenvío inesperado: %+v %v envíos=%d", again, err, svc.sendCalls)
	}
}

func TestPaquetePreservaXMLHistoricoSinDependerDelClienteActual(t *testing.T) {
	uc, _, svc, inv := batchFixture()
	inv.Status = domain.InvoiceOffline
	inv.EmissionType = "OFFLINE"
	inv.Cuf = strPtr("original-cuf")
	inv.Xml = strPtr("<original-firmado/>")
	inv.ContingencyEventId = strPtr("event-1")
	originalDate := inv.IssueDate
	historical := inv.CufdRecord
	historical.ID = "historical"
	historical.Cufd = "CUFD-historical"
	historical.ControlCode = "CTRL-historical"
	current := inv.CufdRecord
	uc.credentials = NewCredentialService(uc.pointOfSaleRepo, &fakeCredCufdStore{existing: &current}, svc, 1)
	inv.CufdRecord = historical
	inv.CufdId = historical.ID
	inv.Customer = domain.Customer{}
	inv.Items = nil
	end := originalDate.Add(time.Minute)
	code := "12345"
	uc.contingencyRepo = &fakeContingencyRepo{latest: &domain.ContingencyEvent{ID: "event-1", PointOfSaleID: inv.PointOfSaleId, StartDate: originalDate.Add(-time.Minute), EndDate: &end, IsSynced: true, SiatEventCode: &code}}
	out, err := uc.EnviarPaquete(context.Background(), inv.CompanyId, inv.PointOfSaleId, PaqueteInput{})
	if err != nil {
		t.Fatal(err)
	}
	doc := svc.lastPackage.Facturas[0]
	if !out.Response.Transaccion || doc.XML != "<original-firmado/>" || doc.Cuf != "original-cuf" || doc.Cufd != historical.Cufd || svc.lastPackage.Cufd != current.Cufd || !doc.FechaEmision.Equal(originalDate) {
		t.Fatalf("se alteró identidad histórica: %+v", doc)
	}
	if svc.lastPackage.CodigoEvento != 12345 || svc.lastPackage.CodigoEmision != 2 {
		t.Fatal("contexto de evento incorrecto")
	}
}

func TestPaqueteFechaFueraEventoNoSeReescribe(t *testing.T) {
	uc, repo, svc, inv := batchFixture()
	inv.Status = domain.InvoiceOffline
	inv.ContingencyEventId = strPtr("event-1")
	date := inv.IssueDate
	start := date.Add(time.Hour)
	end := start.Add(time.Hour)
	code := "99"
	uc.contingencyRepo = &fakeContingencyRepo{latest: &domain.ContingencyEvent{ID: "event-1", PointOfSaleID: inv.PointOfSaleId, StartDate: start, EndDate: &end, IsSynced: true, SiatEventCode: &code}}
	_, err := uc.EnviarPaquete(context.Background(), inv.CompanyId, inv.PointOfSaleId, PaqueteInput{})
	if err == nil || !inv.IssueDate.Equal(date) || svc.sendCalls != 0 || repo.reserves != 0 {
		t.Fatalf("fecha fuera de evento aceptada: %v", err)
	}
}

func TestValidacionUsaSnapshotSinReconstruirFacturas(t *testing.T) {
	for _, kind := range []domain.SentPackageType{domain.PackageTypeMasiva, domain.PackageTypePaquete} {
		t.Run(string(kind), func(t *testing.T) {
			uc, repo, svc, inv := batchFixture()
			emission := 3
			if kind == domain.PackageTypePaquete {
				emission = 2
			}
			p := domain.SentPackage{ID: "batch", CompanyId: inv.CompanyId, PointOfSaleId: inv.PointOfSaleId, Type: kind, InvoiceIDs: []string{inv.ID}, CantidadFacturas: 1, Status: domain.PackageStatusPending, CodigoRecepcion: "receipt", Cuis: "old-cuis", Cufd: "old-cufd", Modalidad: 2, CodigoDocumentoSector: 11, CodigoTipoFactura: 1, CodigoEmision: emission, Layout: "FSEDU"}
			repo.packages[p.ID] = p
			uc.invoiceRepo = nil
			uc.credentials = nil
			inv.Status = domain.InvoiceSent
			svc.onValidate = func(b ports.FiscalBulk, receipt string) {
				if b.Cuis != p.Cuis || b.Cufd != p.Cufd || b.Modalidad != p.Modalidad || b.Layout != p.Layout || b.CodigoDocumentoSector != 11 || b.CodigoEmision != emission || len(b.Facturas) != 0 || receipt != p.CodigoRecepcion {
					t.Fatalf("snapshot alterado: %+v", b)
				}
			}
			validate := uc.ValidarMasiva
			if kind == domain.PackageTypePaquete {
				validate = uc.ValidarPaquete
			}
			out, err := validate(context.Background(), inv.CompanyId, "", PaqueteValidacionInput{BatchID: p.ID})
			if err != nil {
				t.Fatal(err)
			}
			if out.Batches[0].Status != domain.PackageStatusAccepted || svc.prepareCalls != 0 || svc.sendCalls != 0 || inv.Status != domain.InvoiceAccepted {
				t.Fatal("validación incorrecta")
			}
		})
	}
}

func TestValidacionRechazaLoteAjenoOTipoIncorrecto(t *testing.T) {
	for _, other := range []string{"tenant", "type", "pos"} {
		t.Run(other, func(t *testing.T) {
			uc, repo, svc, inv := batchFixture()
			p := domain.SentPackage{ID: "batch", CompanyId: inv.CompanyId, PointOfSaleId: inv.PointOfSaleId, Type: domain.PackageTypeMasiva}
			posID := ""
			switch other {
			case "tenant":
				p.CompanyId = "other"
			case "type":
				p.Type = domain.PackageTypePaquete
			case "pos":
				posID = "other"
			}
			repo.packages[p.ID] = p
			_, err := uc.ValidarMasiva(context.Background(), inv.CompanyId, posID, PaqueteValidacionInput{BatchID: p.ID})
			var nf *domain.NotFoundError
			if !errors.As(err, &nf) || svc.validateCalls != 0 {
				t.Fatalf("err=%v llamadas=%d", err, svc.validateCalls)
			}
		})
	}
}

func TestReservaFallidaNoSeReportaEnviada(t *testing.T) {
	uc, repo, svc, inv := batchFixture()
	repo.reserveErr = errors.New("reserva concurrente")
	out, err := uc.EnviarMasiva(context.Background(), inv.CompanyId, inv.PointOfSaleId, MasivaInput{})
	if err != nil {
		t.Fatal(err)
	}
	if out.Response.Transaccion || out.Batches[0].Status != "NOT_SENT" || out.Batches[0].BatchID != "" || svc.sendCalls != 0 {
		t.Fatalf("resultado=%+v", out.Batches)
	}
}

func TestFalloPersistenciaResultadoConservaRecepcionEnRespuesta(t *testing.T) {
	uc, repo, _, inv := batchFixture()
	repo.updateErr = errors.New("storage failed")
	out, err := uc.EnviarMasiva(context.Background(), inv.CompanyId, inv.PointOfSaleId, MasivaInput{})
	if err != nil {
		t.Fatal(err)
	}
	batch := out.Batches[0]
	if out.Response.Transaccion || batch.Status != domain.PackageStatusUnknown || batch.Response.CodigoRecepcion == "" || !strings.Contains(batch.Error, batch.BatchID) {
		t.Fatalf("resultado=%+v", batch)
	}
	if inv.Status != domain.InvoiceSending || inv.Cuf == nil {
		t.Fatal("perdió la reserva con identidad")
	}
}
