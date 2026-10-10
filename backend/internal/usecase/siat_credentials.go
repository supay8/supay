package usecase

import (
	"context"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

type CuisResultado struct {
	Success bool `json:"success"`
	Data    *ports.CuisResult
}

// SolicitarCUIS fuerza la obtención de un CUIS nuevo (endpoint explícito).
func (uc *SiatUsecase) SolicitarCUIS(ctx context.Context, companyID, posID string) (*CuisResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	resp, err := uc.credentials.RefreshCuis(ctx, company, pointOfSale)
	if err != nil {
		return nil, err
	}
	return &CuisResultado{Success: resp.Transaccion, Data: resp}, nil
}

type CufdResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *ports.CufdResult
}

func (uc *SiatUsecase) SolicitarCUFD(ctx context.Context, companyID, posID string) (*CufdResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	resp, _, err := uc.credentials.RefreshCufd(ctx, company, pointOfSale)
	if err != nil {
		return nil, err
	}
	return &CufdResultado{Company: company, PointOfSale: pointOfSale, Response: resp}, nil
}

// SetupResultado es la respuesta de POST /setup: credenciales resueltas,
// resultado de la sincronización y readiness final.
type SetupResultado struct {
	// Cuis es la respuesta del SIAT solo cuando se solicitó uno nuevo;
	// nil cuando el punto de venta ya tenía CUIS.
	Cuis       *ports.CuisResult        `json:"cuis,omitempty"`
	Cufd       *domain.Cufd             `json:"cufd"`
	Operations []SincronizacionOpResult `json:"operations"`
	Errors     []SincronizacionOpError  `json:"errors,omitempty"`
	Readiness  *domain.CatalogReadiness `json:"readiness,omitempty"`
}

// Setup orquesta el alta completa de un punto de venta en una llamada:
// CUIS (lazy) → sincronización de catálogos → CUFD (lazy) → readiness.
// Es idempotente: re-ejecutarla reutiliza las credenciales vigentes y
// refresca los catálogos.
func (uc *SiatUsecase) Setup(ctx context.Context, companyID, posID string) (*SetupResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}

	out := &SetupResultado{}

	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		resp, err := uc.credentials.RefreshCuis(ctx, company, pointOfSale)
		if err != nil {
			return nil, err
		}
		out.Cuis = resp
	}

	syncRes, err := uc.Sincronizar(ctx, companyID, posID, "")
	if err != nil {
		return nil, err
	}
	out.Operations = syncRes.Operations
	out.Errors = syncRes.Errors

	cufd, err := uc.credentials.EnsureCufd(ctx, company, pointOfSale)
	if err != nil {
		return nil, err
	}
	out.Cufd = cufd

	if readiness, rerr := uc.CatalogReadiness(companyID, posID); rerr == nil {
		out.Readiness = readiness
	}
	return out, nil
}
