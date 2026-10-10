package usecase

import (
	"context"
	"errors"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

type fakeEmissionService struct {
	result        *ports.FiscalResult
	offlineResult *ports.FiscalResult
	docResult     *ports.FiscalDocumentResult
	eventResult   ports.FiscalEventResult
	eventCaptured *ports.FiscalEvent
	err           error
	offlineErr    error
	emitCalls     int
	offlineCalls  int
	captured      *ports.FiscalDocumentQuery
}

func (f *fakeEmissionService) Emit(context.Context, ports.FiscalDocument) (ports.FiscalResult, error) {
	f.emitCalls++
	if f.result == nil {
		return ports.FiscalResult{}, f.err
	}
	return *f.result, f.err
}

func (f *fakeEmissionService) PrepareOffline(context.Context, ports.FiscalDocument) (ports.FiscalResult, error) {
	f.offlineCalls++
	if f.offlineResult == nil {
		if f.offlineErr != nil {
			return ports.FiscalResult{}, f.offlineErr
		}
		return ports.FiscalResult{}, errors.New("offline no configurado")
	}
	return *f.offlineResult, f.offlineErr
}

type fakeContingencyRepo struct {
	latest  *domain.ContingencyEvent
	created []*domain.ContingencyEvent
	updates int
	err     error
}

func (f *fakeContingencyRepo) Create(event *domain.ContingencyEvent) error {
	if f.err != nil {
		return f.err
	}
	if event.ID == "" {
		event.ID = "event-1"
	}
	f.created = append(f.created, event)
	f.latest = event
	return nil
}

func (f *fakeContingencyRepo) Update(event *domain.ContingencyEvent) error {
	if f.err != nil {
		return f.err
	}
	f.latest = event
	f.updates++
	return nil
}

func (f *fakeContingencyRepo) GetLatestByPointOfSale(string) (*domain.ContingencyEvent, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.latest == nil {
		return nil, domain.ErrNotFound
	}
	return f.latest, nil
}

func (f *fakeContingencyRepo) GetBySiatCode(string) (*domain.ContingencyEvent, error) {
	return nil, domain.ErrNotFound
}

func (f *fakeEmissionService) VerifyStatus(context.Context, ports.FiscalDocumentQuery) (ports.FiscalDocumentResult, error) {
	if f.docResult == nil {
		return ports.FiscalDocumentResult{}, f.err
	}
	return *f.docResult, f.err
}

func (f *fakeEmissionService) Annul(_ context.Context, req ports.FiscalDocumentQuery, _ int) (ports.FiscalDocumentResult, error) {
	f.captured = &req
	if f.docResult == nil {
		return ports.FiscalDocumentResult{}, f.err
	}
	return *f.docResult, f.err
}

func (f *fakeEmissionService) RevertAnnul(_ context.Context, req ports.FiscalDocumentQuery) (ports.FiscalDocumentResult, error) {
	f.captured = &req
	if f.docResult == nil {
		return ports.FiscalDocumentResult{}, f.err
	}
	return *f.docResult, f.err
}

func (f *fakeEmissionService) RequestCUIS(context.Context, ports.CredentialRequest) (ports.CuisResult, error) {
	return ports.CuisResult{}, nil
}

func (f *fakeEmissionService) RequestCUFD(context.Context, ports.CredentialRequest) (ports.CufdResult, error) {
	return ports.CufdResult{}, nil
}

func (f *fakeEmissionService) RegisterSignificantEvent(_ context.Context, event ports.FiscalEvent) (ports.FiscalEventResult, error) {
	f.eventCaptured = &event
	return f.eventResult, nil
}

func (f *fakeEmissionService) SendPackage(context.Context, ports.FiscalPackage) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}

func (f *fakeEmissionService) ValidatePackage(context.Context, ports.FiscalPackage, string) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}

func (f *fakeEmissionService) SendBulk(context.Context, ports.FiscalBulk) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}

func (f *fakeEmissionService) ValidateBulk(context.Context, ports.FiscalBulk, string) (ports.FiscalPackageResult, error) {
	return ports.FiscalPackageResult{}, nil
}

func (f *fakeEmissionService) SendPurchases(context.Context, ports.FiscalPurchase) (ports.FiscalPurchaseResult, error) {
	return ports.FiscalPurchaseResult{}, nil
}

func (f *fakeEmissionService) SignXML(context.Context, ports.FiscalSignRequest) (ports.FiscalSignResult, error) {
	return ports.FiscalSignResult{}, nil
}

func (f *fakeEmissionService) EmitAdjustment(context.Context, ports.FiscalAdjustment) (ports.FiscalAdjustmentResult, error) {
	return ports.FiscalAdjustmentResult{}, nil
}

func (f *fakeEmissionService) Synchronize(context.Context, ports.FiscalSyncRequest, ports.FiscalSyncOperation) (ports.FiscalSyncResult, error) {
	return ports.FiscalSyncResult{}, nil
}

type fakeCredentialProvider struct {
	cuisErr   error
	cufd      *domain.Cufd
	cufdErr   error
	cuisCalls int
	cufdCalls int
}

func (f *fakeCredentialProvider) EnsureCuis(_ context.Context, _ *domain.Company, pos *domain.PointOfSale) error {
	f.cuisCalls++
	if f.cuisErr != nil {
		return f.cuisErr
	}
	cuis := "CUIS-LAZY"
	pos.Cuis = &cuis
	return nil
}

func (f *fakeCredentialProvider) EnsureCufd(_ context.Context, _ *domain.Company, _ *domain.PointOfSale) (*domain.Cufd, error) {
	f.cufdCalls++
	return f.cufd, f.cufdErr
}
