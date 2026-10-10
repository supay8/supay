package ports

import (
	"context"
	"io"
	"time"

	"github.com/brandsrx/supay/internal/domain"
)

// AuthRepository agrupa la persistencia que necesita la autenticación humana.
// CreateCompanyForUser debe crear la empresa y la membresía owner de forma
// atómica para no dejar tenants huérfanos.
type AuthRepository interface {
	CreateUser(user *domain.User) error
	GetUserByEmail(email string) (*domain.User, error)
	GetUserByID(id string) (*domain.User, error)
	HasCompanyAccess(userID, companyID string) (bool, error)
	ListCompanies(userID string) ([]domain.UserCompany, error)
	CreateCompanyForUser(userID string, company *domain.Company) error
}

type BranchRepository interface {
	Create(branch *domain.Branch) error
	GetByID(id string) (*domain.Branch, error)
	GetByCompanyAndSucursal(companyID string, codigoSucursal int) (*domain.Branch, error)
	List(companyID string) ([]*domain.Branch, error)
	Update(branch *domain.Branch) error
	Delete(id string) error
}

// CatalogRepository define el contrato para persistir catálogos sincronizados.
type CatalogRepository interface {
	// Replace reemplaza el catálogo de la empresa para el tipo indicado con los
	// valores sincronizados del SIAT (delete + insert en una transacción).
	Replace(companyID, tipo string, items []domain.CatalogItem, syncedAt time.Time) error
	// List devuelve el catálogo vigente de la empresa para el tipo indicado.
	List(companyID, tipo string) ([]*domain.CatalogItem, error)
	// ListAll devuelve todos los catálogos de la empresa agrupados por tipo.
	ListAll(companyID string) (map[string][]*domain.CatalogItem, error)
}

type CatalogSyncStateRepository interface {
	Upsert(state domain.CatalogSyncState) error
	List(companyID, pointOfSaleID string) ([]*domain.CatalogSyncState, error)
}

// CertificateRepository define el contrato para la persistencia de certificados.
type CertificateRepository interface {
	Create(cert *domain.Certificate) error
	GetByID(id string) (*domain.Certificate, error)
	GetActiveByCompany(companyID string) (*domain.Certificate, error)
	ListByCompany(companyID string) ([]*domain.Certificate, error)
	Update(cert *domain.Certificate) error
	Delete(id string) error
}

// CompanyRepository define el contrato para la persistencia
type CompanyRepository interface {
	Create(company *domain.Company) error
	GetByNit(nit string) (*domain.Company, error)
	GetByID(id string) (*domain.Company, error)
	Update(company *domain.Company) error
	Delete(id string) error
}

type ContingencyEventRepository interface {
	Create(e *domain.ContingencyEvent) error
	Update(e *domain.ContingencyEvent) error
	GetLatestByPointOfSale(pointOfSaleID string) (*domain.ContingencyEvent, error)
	GetBySiatCode(siatCode string) (*domain.ContingencyEvent, error)
}

type CufdRepository interface {
	Create(c *domain.Cufd) error
	GetActiveByPos(pointOfSaleID string) (*domain.Cufd, error)
	GetByPosAndWindow(pointOfSaleID string, from, to time.Time) (*domain.Cufd, error)
	DeactivateExpired() error
}

type CuisRepository interface {
	Create(c *domain.Cuis) error
	GetActiveByPos(pointOfSaleID string) (*domain.Cuis, error)
	DeactivateExpired() error
}

// CustomerRepository es append-only: no expone Update ni Delete.
type CustomerRepository interface {
	Create(c *domain.Customer) error
	GetByID(id string) (*domain.Customer, error)
	GetByCompanyAndFiscalIdentity(companyID string, documentType, documentNumber string, complement *string, name string, email string) (*domain.Customer, error)
	List(companyID string) ([]*domain.Customer, error)
}

// EmissionQueue is the application-facing port used by InvoiceUsecase. Its
// PostgreSQL implementation only writes to the outbox; it never calls SIAT.
type EmissionQueue interface {
	EnqueueInvoiceEmission(ctx context.Context, invoiceID, tenantID, cufdID string) (*domain.OutboxEvent, error)
}

// FiscalBatchRepository reserva y reconcilia un envío junto con sus facturas.
// La reserva sobrevive a errores de transporte: un resultado incierto requiere
// conciliación y nunca habilita automáticamente un nuevo envío.
type FiscalBatchRepository interface {
	ReserveBatch(pkg *domain.SentPackage, invoiceIDs []string, expectedStatus domain.InvoiceStatus) error
	UpdateBatch(pkg *domain.SentPackage, invoiceStatus *domain.InvoiceStatus) error
	ListPendingBatchInvoices(companyID, posID string, status domain.InvoiceStatus, eventID *string) ([]*domain.Invoice, error)
}

type InvoiceEmailNotificationRepository interface {
	ClaimPending(ctx context.Context, owner string, limit int, now time.Time, lockTimeout time.Duration) ([]domain.InvoiceEmailNotification, error)
	MarkEnqueued(ctx context.Context, id, owner, taskName string, now time.Time) error
	MarkPublishFailed(ctx context.Context, id, owner, message string, nextAttempt time.Time) error
	ClaimDelivery(ctx context.Context, id string, now time.Time, lockTimeout time.Duration) (*domain.InvoiceEmailNotification, bool, error)
	MarkSent(ctx context.Context, id string, now time.Time) error
	MarkDeliveryFailed(ctx context.Context, id, message string, now time.Time) error
}

type InvoiceEmailTaskDispatcher interface {
	DispatchOnce(context.Context) error
}

type InvoiceEventRepository interface {
	Create(event *domain.InvoiceEvent) error
	List(invoiceID string) ([]*domain.InvoiceEvent, error)
}

type InvoiceFileRepository interface {
	BelongsToCompany(ctx context.Context, companyID, invoiceID string) (bool, error)
	CreateFile(ctx context.Context, file *domain.InvoiceFile) error
	FindFile(ctx context.Context, companyID, invoiceID, kind string) (*domain.InvoiceFile, error)
	DeleteFile(ctx context.Context, companyID, invoiceID, kind, storageKey string) error
}

type InvoiceRepository interface {
	// Create persiste la factura (borrador PENDING) asignando el número
	// correlativo por point_of_sale_id bajo advisory lock (atómico).
	Create(inv *domain.Invoice) error
	GetByID(tenantID, id string) (*domain.Invoice, error)
	GetByIDs(tenantID string, ids []string) ([]*domain.Invoice, error)
	ListByPointOfSale(tenantID, pointOfSaleID string) ([]*domain.Invoice, error)
	// ListFiltered devuelve el listado paginado según el filtro, sin los
	// campos pesados (xml/archivo), junto con el total de coincidencias.
	ListFiltered(filter domain.InvoiceListFilter) ([]*domain.Invoice, int64, error)
	Update(inv *domain.Invoice) error
	TransitionStatus(tenantID, id string, from, to domain.InvoiceStatus, reason domain.InvoiceTransitionReason, fields map[string]any, event *domain.InvoiceEvent) (bool, error)
	// ClaimForEmission marca la factura como SENDING si está PENDING
	// (transición atómica), retornando false si el estado ya no es PENDING.
	ClaimForEmission(tenantID, id string) (bool, error)
	// ReleaseStaleSending revierte a PENDING las facturas atascadas en SENDING
	// durante más de olderThan (crash del proceso, fallo del update final),
	// devolviendo cuántas fueron liberadas.
	ReleaseStaleSending(olderThan time.Duration) (int64, error)
	FindActiveCufdForPointOfSale(pointOfSaleID string, at time.Time) (*domain.Cufd, error)
}

// MaintenanceRepository concentra las consultas cross-tenant reservadas a
// jobs internos. Ningún handler HTTP debe usar este contrato.
type MaintenanceRepository interface {
	ListActiveCredentialTargets(ctx context.Context) ([]domain.CredentialTarget, error)
	ListCertificatesDue(ctx context.Context, dueBefore time.Time) ([]domain.CertificateAlertTarget, error)
	MarkCertificateExpired(ctx context.Context, certificateID string, now time.Time) (bool, error)
	ClaimCertificateNotification(ctx context.Context, certificateID, tenantID string, thresholdDays int, now time.Time) (bool, error)
	MarkCertificateNotificationSent(ctx context.Context, certificateID string, thresholdDays int, deliveredAt time.Time) error
	MarkCertificateNotificationFailed(ctx context.Context, certificateID string, thresholdDays int, message string, failedAt time.Time) error
}

// OutboxRepository is consumed by the dispatcher that relays persistent
// outbox records into River.
type OutboxRepository interface {
	EmissionQueue
	ClaimPending(ctx context.Context, eventType, owner string, limit int, now time.Time, lockTimeout time.Duration) ([]domain.OutboxEvent, error)
	MarkPublished(ctx context.Context, id, owner string, publishedAt time.Time) error
	MarkFailed(ctx context.Context, id, owner, lastError string, nextAttempt time.Time) error
}

// PointOfSaleRepository define el contrato para la persistencia
type PointOfSaleRepository interface {
	// Create persiste un punto de venta calculando automáticamente el
	// codigoPuntoVenta (MAX+1) bajo advisory lock para evitar colisiones
	// bajo concurrencia. Si pos.CodigoPuntoVenta ya es > 0 se respeta ese
	// código local (el índice único valida colisiones).
	Create(pos *domain.PointOfSale) error
	GetByID(id string) (*domain.PointOfSale, error)
	List(companyID string) ([]*domain.PointOfSale, error)
	ListByBranch(branchID string) ([]*domain.PointOfSale, error)
	Update(pos *domain.PointOfSale) error
	Delete(id string) error
}

// SentPackageRepository define el contrato para la persistencia de envíos.
type SentPackageRepository interface {
	Create(pkg *domain.SentPackage) error
	GetByID(id string) (*domain.SentPackage, error)
	GetByCodigoRecepcion(codigoRecepcion string) (*domain.SentPackage, error)
	ListByPointOfSale(pointOfSaleID string) ([]*domain.SentPackage, error)
	ListByCompany(companyID string) ([]*domain.SentPackage, error)
	Update(pkg *domain.SentPackage) error
}

type SiatActividadDocSectorRepository interface {
	// Replace reemplaza la relación actividad-sector de la empresa con los
	// valores sincronizados del SIAT.
	Replace(companyID string, items []domain.SiatActividadDocSector, syncedAt time.Time) error
	// List devuelve todas las relaciones vigentes de la empresa.
	List(companyID string) ([]*domain.SiatActividadDocSector, error)
	// ListByActividad devuelve los sectores habilitados para una actividad.
	ListByActividad(companyID, codigoActividad string) ([]*domain.SiatActividadDocSector, error)
}

type SiatActividadRepository interface {
	// Replace reemplaza el catálogo de actividades de la empresa con los valores
	// sincronizados del SIAT.
	Replace(companyID string, items []domain.SiatActividad, syncedAt time.Time) error
	// List devuelve el catálogo vigente ordenado por código CAEB.
	List(companyID string) ([]*domain.SiatActividad, error)
}

type SiatLeyendaRepository interface {
	// Replace reemplaza el catálogo de leyendas de la empresa con los valores
	// sincronizados del SIAT.
	Replace(companyID string, leyendas []domain.SiatLeyenda, syncedAt time.Time) error
	// List devuelve todas las leyendas vigentes de la empresa.
	List(companyID string) ([]*domain.SiatLeyenda, error)
	// ListByActividad devuelve las leyendas asociadas a una actividad económica.
	ListByActividad(companyID, codigoActividad string) ([]*domain.SiatLeyenda, error)
}

type SinProductRepository interface {
	Replace(companyID string, products []domain.SinProduct, syncedAt time.Time) error
	// List pagina productos activos. codigoActividad=0 no filtra por actividad.
	List(companyID, query string, codigoActividad int64, limit, offset int) ([]*domain.SinProduct, int64, error)
	ListAll(companyID string) ([]*domain.SinProduct, error)
	GetByCode(companyID string, code int64) (*domain.SinProduct, error)
}

type Storage interface {
	Put(context.Context, string, io.Reader, domain.PutOptions) (domain.ObjectInfo, error)
	Get(context.Context, string) (io.ReadCloser, domain.ObjectInfo, error)
	Stat(context.Context, string) (domain.ObjectInfo, error)
	Delete(context.Context, string) error
	PresignGet(context.Context, string, time.Duration, string) (string, error)
}

type TipoPuntoVentaRepository interface {
	// Replace reemplaza el catálogo de tipos de punto de venta de la empresa
	// con los valores sincronizados del SIAT.
	Replace(companyID string, tipos []domain.TipoPuntoVenta, syncedAt time.Time) error
	// List devuelve el catálogo vigente de la empresa ordenado por clasificador.
	List(companyID string) ([]*domain.TipoPuntoVenta, error)
	// FindByClasificador busca un tipo por su código oficial.
	FindByClasificador(companyID string, codigoClasificador int) (*domain.TipoPuntoVenta, error)
}
