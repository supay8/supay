package usecase

import (
	"context"
	"errors"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	siat "github.com/brandsrx/supay/internal/siat"
)

type SiatManager struct {
	cuisRepo domain.CuisRepository
	cufdRepo domain.CufdRepository
	client   *siat.Client
}

func NewSiatManager(cuisRepo domain.CuisRepository, cufdRepo domain.CufdRepository, client *siat.Client) *SiatManager {
	return &SiatManager{cuisRepo: cuisRepo, cufdRepo: cufdRepo, client: client}
}

// EnsureActiveCuis ensures there is an active CUIS for the POS, otherwise requests one from SIAT
func (m *SiatManager) EnsureActiveCuis(ctx context.Context, pointOfSaleID string, request siat.SolicitudCuis) (*domain.Cuis, error) {
	// check repo
	if c, err := m.cuisRepo.GetActiveByPos(pointOfSaleID); err == nil && c != nil {
		return c, nil
	}

	// request CUIS from SIAT
	resp, err := siat.NewCuisService(m.client).SolicitarCUIS(ctx, request)
	if err != nil {
		return nil, err
	}
	if resp == nil || resp.Codigo == "" {
		return nil, errors.New("siat cuis: respuesta inválida")
	}

	now := time.Now()
	validTo := resp.FechaVigencia.Time
	if validTo.IsZero() {
		validTo = now.Add(6 * time.Hour)
	}
	cuis := &domain.Cuis{
		PointOfSaleID: pointOfSaleID,
		Cuis:          resp.Codigo,
		ValidFrom:     now,
		ValidTo:       validTo,
		Active:        true,
		CreatedAt:     now,
	}
	if err := m.cuisRepo.Create(cuis); err != nil {
		return nil, err
	}
	return cuis, nil
}
