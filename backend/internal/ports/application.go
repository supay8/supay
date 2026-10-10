package ports

import (
	"context"

	"github.com/brandsrx/supay/internal/domain"
)

type CredentialPosStore interface {
	Update(pos *domain.PointOfSale) error
}

type CredentialCufdStore interface {
	Create(c *domain.Cufd) error
	GetActiveByPos(pointOfSaleID string) (*domain.Cufd, error)
}

type CredentialProvider interface {
	EnsureCuis(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) error
	EnsureCufd(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*domain.Cufd, error)
}

type CredentialMaintainer interface {
	EnsureCuis(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) error
	EnsureCufd(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*domain.Cufd, error)
	RefreshCuis(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*CuisResult, error)
	RefreshCufd(ctx context.Context, company *domain.Company, pos *domain.PointOfSale) (*CufdResult, *domain.Cufd, error)
}

type PdfGenerator interface {
	GenerateAndPersist(ctx context.Context, invoiceID string)
}

// StrictPdfGenerator reports failures to the synchronous emission pipeline.
type StrictPdfGenerator interface {
	GenerateAndPersistStrict(context.Context, string) error
}
