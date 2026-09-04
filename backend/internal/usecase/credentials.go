package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

// CredentialPosStore y CredentialCufdStore son los contratos mínimos de
// persistencia que necesita el servicio de credenciales. Los repositorios
// reales los satisfacen; los tests usan fakes chicos.
type CredentialPosStore interface {
	Update(pos *domain.PointOfSale) error
}

type CredentialCufdStore interface {
	Create(c *domain.Cufd) error
	GetActiveByPos(pointOfSaleID string) (*domain.Cufd, error)
}

// CredentialProvider resuelve credenciales vigentes para un punto de venta,
// solicitándolas al SIAT on-demand (lazy) cuando faltan o vencieron. La
// emisión depende de esta interfaz, no del servicio concreto.
type CredentialProvider interface {
	EnsureCuis(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) error
	EnsureCufd(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*domain.Cufd, error)
}

// CredentialService implementa CredentialProvider centralizando la lógica de
// CUIS/CUFD que antes vivía duplicada en SiatUsecase: construir la solicitud
// con la identidad de la empresa/PV, llamar al SIAT y persistir el resultado.
type CredentialService struct {
	posRepo     CredentialPosStore
	cufdRepo    CredentialCufdStore
	siatService ports.FiscalService
	modalidad   int
	provider    siat.SiatClientProvider
}

func NewCredentialService(posRepo CredentialPosStore, cufdRepo CredentialCufdStore, siatService ports.FiscalService, modalidad int) *CredentialService {
	return &CredentialService{posRepo: posRepo, cufdRepo: cufdRepo, siatService: siatService, modalidad: modalidad}
}

// NewCredentialServiceWithProvider crea el servicio con resolución por empresa via provider.
func NewCredentialServiceWithProvider(posRepo CredentialPosStore, cufdRepo CredentialCufdStore, provider siat.SiatClientProvider, modalidad int) *CredentialService {
	return &CredentialService{posRepo: posRepo, cufdRepo: cufdRepo, provider: provider, modalidad: modalidad}
}

// SetProvider inyecta el provider multi-tenant después de construir (para wiring sin ciclo).
func (s *CredentialService) SetProvider(p siat.SiatClientProvider) { s.provider = p }

func (s *CredentialService) effectiveModalidad() int {
	if s.modalidad <= 0 {
		return siat.ModalidadElectronica
	}
	return s.modalidad
}

func (s *CredentialService) effectiveModalidadForCompany(company *domain.Company) int {
	if company != nil && company.Modalidad != 0 {
		return company.Modalidad
	}
	return s.effectiveModalidad()
}

func (s *CredentialService) resolveClient(ctx context.Context, company *domain.Company) (ports.FiscalService, error) {
	if s.provider != nil && company != nil && company.ID != "" {
		if svc, err := s.provider.GetForCompany(ctx, company.ID); err == nil && svc != nil {
			return siat.NewFiscalAdapter(svc), nil
		} else if s.siatService == nil {
			if err != nil {
				return nil, err
			}
			return nil, ErrSiatNoDisponible
		}
	}
	if s.siatService == nil {
		return nil, ErrSiatNoDisponible
	}
	return s.siatService, nil
}

// EnsureCuis garantiza que el punto de venta tenga un CUIS persistido. Si ya
// tiene uno, no hace nada (el CUIS es vigente por meses).
func (s *CredentialService) EnsureCuis(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) error {
	if pos.Cuis != nil && *pos.Cuis != "" {
		return nil
	}
	resp, err := s.requestCuis(ctx, company, pos)
	if err != nil {
		return err
	}
	pos.Cuis = &resp.Codigo
	now := time.Now().In(siat.LaPaz)
	pos.CuisCreatedAt = &now
	if err := s.posRepo.Update(pos); err != nil {
		slog.Error("no se pudo persistir el cuis en el punto de venta", "pos_id", pos.ID, "error", err)
		return domain.NewConflictError("no se pudo persistir el cuis en el punto de venta")
	}
	return nil
}

// EnsureCufd devuelve el CUFD vigente del punto de venta, solicitando uno
// nuevo al SIAT cuando no existe o venció (validez ~24h).
func (s *CredentialService) EnsureCufd(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*domain.Cufd, error) {
	if cufd, err := s.cufdRepo.GetActiveByPos(pos.ID); err == nil && cufd != nil {
		return cufd, nil
	}
	resp, err := s.requestCufd(ctx, company, pos)
	if err != nil {
		return nil, err
	}
	now := time.Now().In(siat.LaPaz)
	cufd := &domain.Cufd{
		PointOfSaleID: pos.ID,
		Cufd:          resp.Codigo,
		ControlCode:   resp.CodigoControl,
		Direccion:     resp.Direccion,
		ValidFrom:     now,
		ValidTo:       resp.FechaVigencia,
		Active:        true,
	}
	if err := s.cufdRepo.Create(cufd); err != nil {
		slog.Error("no se pudo persistir el cufd", "pos_id", pos.ID, "error", err)
		return nil, domain.NewConflictError("no se pudo persistir el cufd")
	}
	return cufd, nil
}

// RefreshCuis solicita un CUIS nuevo aunque el punto de venta ya tenga uno
// (endpoint explícito POST /point-of-sales/{id}/cuis).
func (s *CredentialService) RefreshCuis(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*ports.CuisResult, error) {
	resp, err := s.requestCuis(ctx, company, pos)
	if err != nil {
		return nil, err
	}
	pos.Cuis = &resp.Codigo
	now := time.Now().In(siat.LaPaz)
	pos.CuisCreatedAt = &now
	if err := s.posRepo.Update(pos); err != nil {
		slog.Error("no se pudo persistir el cuis en el punto de venta", "pos_id", pos.ID, "error", err)
		return nil, domain.NewConflictError("no se pudo persistir el cuis en el punto de venta")
	}
	return resp, nil
}

// RefreshCufd solicita un CUFD nuevo aunque exista uno vigente (endpoint
// explícito POST /point-of-sales/{id}/cufd).
func (s *CredentialService) RefreshCufd(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*ports.CufdResult, *domain.Cufd, error) {
	if err := s.EnsureCuis(ctx, company, pos); err != nil {
		return nil, nil, err
	}
	resp, err := s.requestCufd(ctx, company, pos)
	if err != nil {
		return nil, nil, err
	}
	now := time.Now().In(siat.LaPaz)
	cufd := &domain.Cufd{
		PointOfSaleID: pos.ID,
		Cufd:          resp.Codigo,
		ControlCode:   resp.CodigoControl,
		Direccion:     resp.Direccion,
		ValidFrom:     now,
		ValidTo:       resp.FechaVigencia,
		Active:        true,
	}
	if err := s.cufdRepo.Create(cufd); err != nil {
		slog.Error("no se pudo persistir el cufd", "pos_id", pos.ID, "error", err)
		return nil, nil, domain.NewConflictError("no se pudo persistir el cufd")
	}
	return resp, cufd, nil
}

func (s *CredentialService) requestCuis(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*ports.CuisResult, error) {
	client, err := s.resolveClient(ctx, company)
	if err != nil {
		return nil, err
	}
	req := ports.CredentialRequest{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pos.CodigoSucursal,
		CodigoModalidad:  s.effectiveModalidadForCompany(company),
		CodigoPuntoVenta: resolveCodigoPuntoVenta(pos),
	}
	if pos.Cuis != nil && *pos.Cuis != "" {
		req.Cuis = *pos.Cuis
	}
	resp, err := client.RequestCUIS(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Codigo == "" {
		return nil, domain.NewConflictError("el siat no entregó un cuis válido para el punto de venta")
	}
	return &resp, nil
}

func (s *CredentialService) requestCufd(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*ports.CufdResult, error) {
	client, err := s.resolveClient(ctx, company)
	if err != nil {
		return nil, err
	}
	if err := s.EnsureCuis(ctx, company, pos); err != nil {
		return nil, err
	}
	req := ports.CredentialRequest{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pos.CodigoSucursal,
		CodigoModalidad:  s.effectiveModalidadForCompany(company),
		CodigoPuntoVenta: resolveCodigoPuntoVenta(pos),
		Cuis:             *pos.Cuis,
	}
	resp, err := client.RequestCUFD(ctx, req)
	if err != nil {
		return nil, err
	}
	if resp.Codigo == "" {
		return nil, domain.NewConflictError("el siat no entregó un cufd válido para el punto de venta")
	}
	return &resp, nil
}
