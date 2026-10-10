package usecase

import (
	"context"
	"errors"
	"testing"

	"github.com/brandsrx/supay/internal/adapters/siat/batch"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

type preparationRepo struct {
	*fakeInvoiceRepo
	store func(context.Context, *domain.Invoice) error
}

func (r *preparationRepo) StorePreparedEmission(ctx context.Context, inv *domain.Invoice) error {
	if err := r.store(ctx, inv); err != nil {
		return err
	}
	stored := r.invoices[inv.ID]
	stored.Cuf, stored.XmlHash = inv.Cuf, inv.XmlHash
	stored.Archivo, stored.HashArchivo, stored.CufdId, stored.CufdRecord = inv.Archivo, inv.HashArchivo, inv.CufdId, inv.CufdRecord
	return nil
}

type pipelineService struct {
	fakeEmissionService
	prepare  func(context.Context, ports.FiscalDocument) (ports.FiscalResult, error)
	dispatch func(context.Context, ports.FiscalDocument, ports.FiscalResult) (ports.FiscalResult, error)
}

func (s *pipelineService) PrepareEmission(ctx context.Context, doc ports.FiscalDocument) (ports.FiscalResult, error) {
	return s.prepare(ctx, doc)
}
func (s *pipelineService) DispatchEmission(ctx context.Context, doc ports.FiscalDocument, result ports.FiscalResult) (ports.FiscalResult, error) {
	return s.dispatch(ctx, doc, result)
}
func preparedFixture(t *testing.T) ports.FiscalResult {
	t.Helper()
	xml := "<signed>immutable bytes</signed>"
	packed, err := (batch.Packer{}).PackDocument([]byte(xml))
	if err != nil {
		t.Fatal(err)
	}
	return ports.FiscalResult{Cuf: "CUF-ONLINE", Xml: xml, Archivo: packed.Archive, XmlHash: packed.Hash}
}

func TestEmissionPersistsBeforeDispatchAndReusesPreparationOnRetry(t *testing.T) {
	base := newFakeInvoiceRepo()
	inv := testInvoice()
	base.invoices[inv.ID] = inv
	prepared := preparedFixture(t)
	preparations, dispatches, stores := 0, 0, 0
	svc := &pipelineService{}
	uc := newTestUsecase(base, &fakeCatalogRepo{}, svc)
	uc.invoiceRepo = &preparationRepo{fakeInvoiceRepo: base, store: func(ctx context.Context, inv *domain.Invoice) error {
		stores++
		xml, file, err := uc.fileService.ReadAll(ctx, inv.CompanyId, inv.ID, "xml")
		if err != nil || string(xml) != prepared.Xml || inv.XmlHash == nil || *inv.XmlHash != file.SHA256 {
			t.Fatalf("storage must precede SQL: file=%+v err=%v", file, err)
		}
		return nil
	}}
	svc.prepare = func(context.Context, ports.FiscalDocument) (ports.FiscalResult, error) {
		preparations++
		return prepared, nil
	}
	transportErr := errors.New("retryable SOAP protocol error")
	svc.dispatch = func(_ context.Context, _ ports.FiscalDocument, got ports.FiscalResult) (ports.FiscalResult, error) {
		dispatches++
		if stores != dispatches || got.Xml != prepared.Xml || got.Archivo != prepared.Archivo {
			t.Fatal("dispatch must use durably stored bytes")
		}
		if dispatches == 1 {
			return ports.FiscalResult{}, transportErr
		}
		got.Transaccion = true
		got.CodigoEstado = 908
		return got, nil
	}
	if _, err := uc.Emit(invoiceContext(), inv.ID); !errors.Is(err, transportErr) {
		t.Fatalf("expected transport failure, got %v", err)
	}
	got, err := uc.Emit(invoiceContext(), inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.InvoiceAccepted || preparations != 1 || dispatches != 2 {
		t.Fatalf("status=%s preparations=%d dispatches=%d", got.Status, preparations, dispatches)
	}
}

func TestEmissionPreparationFailureNeverDispatches(t *testing.T) {
	for _, stage := range []string{"serialize", "storage", "database"} {
		t.Run(stage, func(t *testing.T) {
			base := newFakeInvoiceRepo()
			inv := testInvoice()
			base.invoices[inv.ID] = inv
			sentinel := errors.New("preparation failed")
			prepared := preparedFixture(t)
			svc := &pipelineService{prepare: func(context.Context, ports.FiscalDocument) (ports.FiscalResult, error) {
				if stage == "serialize" {
					return ports.FiscalResult{}, sentinel
				}
				return prepared, nil
			}, dispatch: func(context.Context, ports.FiscalDocument, ports.FiscalResult) (ports.FiscalResult, error) {
				t.Fatal("SOAP called before durable preparation")
				return ports.FiscalResult{}, nil
			}}
			uc := newTestUsecase(base, &fakeCatalogRepo{}, svc)
			files := uc.fileService
			if stage == "storage" {
				uc.fileService = nil
			}
			uc.invoiceRepo = &preparationRepo{fakeInvoiceRepo: base, store: func(context.Context, *domain.Invoice) error {
				if stage == "database" {
					return sentinel
				}
				t.Fatal("SQL must follow successful serialization and storage")
				return nil
			}}
			if _, err := uc.Emit(invoiceContext(), inv.ID); err == nil {
				t.Fatal("failure swallowed")
			}
			if stage == "database" {
				if _, _, err := files.ReadAll(context.Background(), inv.CompanyId, inv.ID, "xml"); !errors.Is(err, domain.ErrNotFound) {
					t.Fatalf("uncommitted object must be compensated: %v", err)
				}
			}
		})
	}
}

func TestPreparedOnlineEmissionCanBecomeOfflineWithoutFileCollision(t *testing.T) {
	base := newFakeInvoiceRepo()
	inv := testInvoice()
	base.invoices[inv.ID] = inv
	prepared := preparedFixture(t)
	svc := &pipelineService{fakeEmissionService: fakeEmissionService{offlineResult: &ports.FiscalResult{Cuf: "CUF-OFFLINE", Xml: "<offline/>", Archivo: "offline", XmlHash: "offline-hash"}}, prepare: func(context.Context, ports.FiscalDocument) (ports.FiscalResult, error) { return prepared, nil }, dispatch: func(context.Context, ports.FiscalDocument, ports.FiscalResult) (ports.FiscalResult, error) {
		return ports.FiscalResult{}, context.DeadlineExceeded
	}}
	uc := newTestUsecase(base, &fakeCatalogRepo{}, svc)
	uc.SetContingencyRepository(&fakeContingencyRepo{})
	uc.invoiceRepo = &preparationRepo{fakeInvoiceRepo: base, store: func(context.Context, *domain.Invoice) error { return nil }}
	got, err := uc.Emit(invoiceContext(), inv.ID)
	if err != nil {
		t.Fatal(err)
	}
	data, file, err := uc.fileService.ReadAll(context.Background(), inv.CompanyId, inv.ID, "xml")
	if err != nil || got.Status != domain.InvoiceOffline || string(data) != "<offline/>" || file.StorageKey == "" {
		t.Fatalf("offline result=%+v file=%+v err=%v", got, file, err)
	}
}

func TestInvalidSectorPayloadFailsBeforeCredentialsOrSOAP(t *testing.T) {
	base := newFakeInvoiceRepo()
	inv := testInvoice()
	inv.SectorData = []byte(`{"unknown_field":1}`)
	base.invoices[inv.ID] = inv
	credentials := &fakeCredentialProvider{}
	svc := &fakeEmissionService{}
	uc := newTestUsecase(base, &fakeCatalogRepo{}, svc)
	uc.credentials = credentials
	_, err := uc.Emit(invoiceContext(), inv.ID)
	var badRequest *domain.BadRequestError
	if !errors.As(err, &badRequest) {
		t.Fatalf("expected HTTP400 domain error, got %v", err)
	}
	if credentials.cuisCalls != 0 || credentials.cufdCalls != 0 || svc.emitCalls != 0 || base.claimCalls != 0 {
		t.Fatal("invalid payload caused external work")
	}
}

func TestPreparationTimeoutDoesNotActivateConnectivityContingency(t *testing.T) {
	base := newFakeInvoiceRepo()
	inv := testInvoice()
	base.invoices[inv.ID] = inv
	svc := &pipelineService{prepare: func(context.Context, ports.FiscalDocument) (ports.FiscalResult, error) {
		return ports.FiscalResult{}, context.DeadlineExceeded
	}, dispatch: func(context.Context, ports.FiscalDocument, ports.FiscalResult) (ports.FiscalResult, error) {
		t.Fatal("dispatch after preparation timeout")
		return ports.FiscalResult{}, nil
	}}
	uc := newTestUsecase(base, &fakeCatalogRepo{}, svc)
	uc.SetContingencyRepository(&fakeContingencyRepo{})
	uc.invoiceRepo = &preparationRepo{fakeInvoiceRepo: base, store: func(context.Context, *domain.Invoice) error { t.Fatal("store after preparation timeout"); return nil }}
	if _, err := uc.Emit(invoiceContext(), inv.ID); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lost timeout: %v", err)
	}
	if svc.offlineCalls != 0 {
		t.Fatal("local preparation timeout is not a SIAT connectivity failure")
	}
}
