package postgres

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"slices"
	"sort"
	"strings"
	"time"
)

type PostgresSentPackageRepository struct {
	db *gorm.DB
}

var _ domain.FiscalBatchRepository = (*PostgresSentPackageRepository)(nil)

func NewPostgresSentPackageRepository(db *gorm.DB) domain.SentPackageRepository {
	return &PostgresSentPackageRepository{db: db}
}

func (r *PostgresSentPackageRepository) Create(pkg *domain.SentPackage) error {
	dbModel := toModelSentPackage(pkg)
	if dbModel.ID == "" {
		dbModel.ID = uuid.NewString()
	}
	if err := r.db.Create(&dbModel).Error; err != nil {
		return err
	}
	pkg.ID = dbModel.ID
	pkg.CreatedAt = dbModel.CreatedAt
	return nil
}

func toModelSentPackage(pkg *domain.SentPackage) models.SentPackage {
	m := models.SentPackage{
		ID:                    pkg.ID,
		CompanyId:             pkg.CompanyId,
		PointOfSaleId:         pkg.PointOfSaleId,
		Type:                  string(pkg.Type),
		CodigoRecepcion:       pkg.CodigoRecepcion,
		HashArchivo:           pkg.HashArchivo,
		CantidadFacturas:      pkg.CantidadFacturas,
		CodigoDocumentoSector: pkg.CodigoDocumentoSector,
		CodigoTipoFactura:     pkg.CodigoTipoFactura,
		CodigoEmision:         pkg.CodigoEmision,
		CodigoEvento:          pkg.CodigoEvento,
		ContingencyEventId:    pkg.ContingencyEventId,
		Status:                string(pkg.Status),
		Mensajes:              pkg.Mensajes,
		XmlHash:               pkg.XmlHash,
		SentAt:                pkg.SentAt,
		ValidatedAt:           pkg.ValidatedAt,
		CreatedAt:             pkg.CreatedAt,
		Modalidad:             pkg.Modalidad,
		Layout:                pkg.Layout,
		Cufd:                  pkg.Cufd,
		Cuis:                  pkg.Cuis,
	}
	if pkg.CufdID != "" {
		m.CufdID = &pkg.CufdID
	}
	return m
}

func (r *PostgresSentPackageRepository) GetByID(id string) (*domain.SentPackage, error) {
	var dbModel models.SentPackage
	if err := r.db.Where("id = ?", id).First(&dbModel).Error; err != nil {
		return nil, err
	}
	pkg := toDomainSentPackage(&dbModel)
	return pkg, r.loadMembership([]*domain.SentPackage{pkg})
}

func (r *PostgresSentPackageRepository) GetByCodigoRecepcion(codigoRecepcion string) (*domain.SentPackage, error) {
	if codigoRecepcion == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var dbModel models.SentPackage
	if err := r.db.Where("codigo_recepcion = ?", codigoRecepcion).First(&dbModel).Error; err != nil {
		return nil, err
	}
	pkg := toDomainSentPackage(&dbModel)
	return pkg, r.loadMembership([]*domain.SentPackage{pkg})
}

func (r *PostgresSentPackageRepository) ListByPointOfSale(pointOfSaleID string) ([]*domain.SentPackage, error) {
	var dbModels []models.SentPackage
	if err := r.db.Where("point_of_sale_id = ?", pointOfSaleID).
		Order("created_at DESC").
		Find(&dbModels).Error; err != nil {
		return nil, err
	}
	pkgs := toDomainSentPackages(dbModels)
	return pkgs, r.loadMembership(pkgs)
}

func (r *PostgresSentPackageRepository) ListByCompany(companyID string) ([]*domain.SentPackage, error) {
	var dbModels []models.SentPackage
	if err := r.db.Where("tenant_id = ?", companyID).
		Order("created_at DESC").
		Find(&dbModels).Error; err != nil {
		return nil, err
	}
	pkgs := toDomainSentPackages(dbModels)
	return pkgs, r.loadMembership(pkgs)
}

func (r *PostgresSentPackageRepository) Update(pkg *domain.SentPackage) error {
	return r.UpdateBatch(pkg, nil)
}

func toDomainSentPackage(dbModel *models.SentPackage) *domain.SentPackage {
	pkg := &domain.SentPackage{
		ID:                    dbModel.ID,
		CompanyId:             dbModel.CompanyId,
		PointOfSaleId:         dbModel.PointOfSaleId,
		Type:                  domain.SentPackageType(dbModel.Type),
		CodigoRecepcion:       dbModel.CodigoRecepcion,
		HashArchivo:           dbModel.HashArchivo,
		CantidadFacturas:      dbModel.CantidadFacturas,
		CodigoDocumentoSector: dbModel.CodigoDocumentoSector,
		CodigoTipoFactura:     dbModel.CodigoTipoFactura,
		CodigoEmision:         dbModel.CodigoEmision,
		CodigoEvento:          dbModel.CodigoEvento,
		ContingencyEventId:    dbModel.ContingencyEventId,
		Status:                domain.SentPackageStatus(dbModel.Status),
		Mensajes:              dbModel.Mensajes,
		XmlHash:               dbModel.XmlHash,
		SentAt:                dbModel.SentAt,
		ValidatedAt:           dbModel.ValidatedAt,
		CreatedAt:             dbModel.CreatedAt,
		Modalidad:             dbModel.Modalidad,
		Layout:                dbModel.Layout,
		Cufd:                  dbModel.Cufd,
		Cuis:                  dbModel.Cuis,
	}
	if dbModel.CufdID != nil {
		pkg.CufdID = *dbModel.CufdID
	}
	return pkg
}

func toDomainSentPackages(dbModels []models.SentPackage) []*domain.SentPackage {
	out := make([]*domain.SentPackage, 0, len(dbModels))
	for i := range dbModels {
		out = append(out, toDomainSentPackage(&dbModels[i]))
	}
	return out
}

func (r *PostgresSentPackageRepository) loadMembership(pkgs []*domain.SentPackage) error {
	if len(pkgs) == 0 {
		return nil
	}
	ids := make([]string, 0, len(pkgs))
	byID := make(map[string]*domain.SentPackage, len(pkgs))
	for _, pkg := range pkgs {
		ids = append(ids, pkg.ID)
		byID[pkg.ID] = pkg
		pkg.InvoiceIDs = []string{}
	}
	var members []models.SentPackageInvoice
	if err := r.db.Where("sent_package_id IN ?", ids).Order("position").Find(&members).Error; err != nil {
		return err
	}
	for _, member := range members {
		pkg := byID[member.SentPackageID]
		pkg.InvoiceIDs = append(pkg.InvoiceIDs, member.InvoiceID)
	}
	return nil
}

func (r *PostgresSentPackageRepository) ListPendingBatchInvoices(companyID, posID string, status domain.InvoiceStatus, eventID *string) ([]*domain.Invoice, error) {
	if status != domain.InvoicePending && status != domain.InvoiceOffline {
		return nil, fmt.Errorf("estado no reservable: %s", status)
	}
	query := r.db.Where("tenant_id = ? AND point_of_sale_id = ? AND status = ?", companyID, posID, status).
		Where("(siat_reception_code IS NULL OR btrim(siat_reception_code) = '')").
		Where("NOT EXISTS (SELECT 1 FROM sent_package_invoices b WHERE b.invoice_id = invoices.id)")
	if status == domain.InvoicePending {
		query = query.Where("(cuf IS NULL OR btrim(cuf) = '')").
			Where("emission_type NOT IN ?", []models.EmissionType{models.EmissionOffline, models.EmissionMasiva})
	}
	if eventID != nil {
		query = query.Where("contingency_event_id = ?", *eventID)
	} else {
		query = query.Where("contingency_event_id IS NULL")
	}
	var rows []models.Invoice
	if err := query.Preload("Items").Preload("CufdRecord").
		Order("invoice_number, id").Find(&rows).Error; err != nil {
		return nil, err
	}
	invoices := make([]*domain.Invoice, 0, len(rows))
	for i := range rows {
		invoices = append(invoices, toDomainInvoice(&rows[i]))
	}
	return invoices, nil
}

func (r *PostgresSentPackageRepository) ReserveBatch(pkg *domain.SentPackage, invoiceIDs []string, expectedStatus domain.InvoiceStatus) error {
	if pkg == nil || len(invoiceIDs) == 0 {
		return fmt.Errorf("el lote debe contener facturas")
	}
	if expectedStatus != domain.InvoicePending && expectedStatus != domain.InvoiceOffline {
		return fmt.Errorf("estado no reservable: %s", expectedStatus)
	}
	if pkg.CodigoRecepcion != "" || pkg.CantidadFacturas != len(invoiceIDs) {
		return fmt.Errorf("metadatos de reserva de lote inconsistentes")
	}
	if (pkg.Type != domain.PackageTypeMasiva || expectedStatus != domain.InvoicePending) &&
		(pkg.Type != domain.PackageTypePaquete || expectedStatus != domain.InvoiceOffline) {
		return fmt.Errorf("tipo de lote incompatible con el estado de sus facturas")
	}
	if (pkg.Type == domain.PackageTypeMasiva && len(pkg.Documents) != len(invoiceIDs)) ||
		(len(pkg.Documents) > 0 && len(pkg.Documents) != len(invoiceIDs)) {
		return fmt.Errorf("documentos preparados inconsistentes con el lote")
	}
	documents := make(map[string]domain.BatchInvoiceDocument, len(pkg.Documents))
	for i, document := range pkg.Documents {
		if strings.TrimSpace(document.Cuf) == "" || strings.TrimSpace(document.Xml) == "" {
			return fmt.Errorf("el documento preparado requiere CUF y XML")
		}
		hash := fmt.Sprintf("%x", sha256.Sum256([]byte(document.Xml)))
		if document.XmlHash != "" && document.XmlHash != hash {
			return fmt.Errorf("hash del XML preparado inconsistente")
		}
		document.XmlHash = hash
		documents[invoiceIDs[i]] = document
	}
	orderedIDs := slices.Clone(invoiceIDs)
	sort.Strings(orderedIDs)
	for i, id := range orderedIDs {
		if id == "" || (i > 0 && orderedIDs[i-1] == id) {
			return fmt.Errorf("facturas vacías o repetidas en el lote")
		}
	}
	m := toModelSentPackage(pkg)
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	m.Status = string(domain.PackageStatusSending)
	if m.SentAt.IsZero() {
		m.SentAt = time.Now()
	}
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var invoices []models.Invoice
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", orderedIDs).
			Order("id").Find(&invoices).Error; err != nil {
			return err
		}
		if len(invoices) != len(orderedIDs) {
			return fmt.Errorf("faltan facturas del lote")
		}
		for _, invoice := range invoices {
			if invoice.CompanyId != pkg.CompanyId || invoice.PointOfSaleId != pkg.PointOfSaleId ||
				domain.InvoiceStatus(invoice.Status) != expectedStatus ||
				(invoice.SiatReceptionCode != nil && strings.TrimSpace(*invoice.SiatReceptionCode) != "") ||
				!sameOptionalString(invoice.ContingencyEventId, pkg.ContingencyEventId) {
				return fmt.Errorf("factura %s no disponible para el lote", invoice.ID)
			}
			if expectedStatus == domain.InvoicePending &&
				((invoice.Cuf != nil && strings.TrimSpace(*invoice.Cuf) != "") ||
					invoice.EmissionType == models.EmissionOffline || invoice.EmissionType == models.EmissionMasiva) {
				return fmt.Errorf("factura %s ya tiene una identidad fiscal preparada", invoice.ID)
			}
			if invoice.CodigoDocumentoSector > 0 && invoice.CodigoDocumentoSector != pkg.CodigoDocumentoSector ||
				invoice.CodigoTipoFactura > 0 && invoice.CodigoTipoFactura != pkg.CodigoTipoFactura ||
				invoice.Modalidad > 0 && invoice.Modalidad != pkg.Modalidad ||
				invoice.Layout != "" && invoice.Layout != pkg.Layout {
				return fmt.Errorf("factura %s tiene metadatos fiscales distintos al lote", invoice.ID)
			}
			if document, ok := documents[invoice.ID]; ok && invoice.Cuf != nil &&
				strings.TrimSpace(*invoice.Cuf) != "" && *invoice.Cuf != document.Cuf {
				return fmt.Errorf("el CUF de la factura %s es inmutable", invoice.ID)
			}
		}
		var reserved int64
		if err := tx.Model(&models.SentPackageInvoice{}).Where("invoice_id IN ?", orderedIDs).Count(&reserved).Error; err != nil {
			return err
		}
		if reserved != 0 {
			return fmt.Errorf("el lote contiene facturas ya reservadas")
		}
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		members := make([]models.SentPackageInvoice, len(invoiceIDs))
		for i, id := range invoiceIDs {
			members[i] = models.SentPackageInvoice{TenantID: m.CompanyId, InvoiceID: id, SentPackageID: m.ID, Position: i}
		}
		if err := tx.Create(&members).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Invoice{}).Where("id IN ?", orderedIDs).
			Update("status", models.StatusSending).Error; err != nil {
			return err
		}
		for _, invoice := range invoices {
			if document, ok := documents[invoice.ID]; ok {
				fields := map[string]any{"cuf": document.Cuf, "xml": document.Xml, "xml_hash": document.XmlHash}
				if document.Archivo != "" {
					fields["archivo"], fields["hash_archivo"] = document.Archivo, document.HashArchivo
				}
				if pkg.Type == domain.PackageTypeMasiva {
					fields["emission_type"] = models.EmissionMasiva
					fields["cufd_id"] = pkg.CufdID
					fields["modalidad"] = pkg.Modalidad
					fields["layout"] = pkg.Layout
					fields["codigo_tipo_factura"] = pkg.CodigoTipoFactura
				}
				if err := tx.Model(&models.Invoice{}).Where("id = ?", invoice.ID).Updates(fields).Error; err != nil {
					return err
				}
				if err := persistInvoiceDocuments(tx, invoice.ID, fields); err != nil {
					return err
				}
			}
			if err := recordBatchTransition(tx, invoice, m.ID, domain.InvoiceSending, "BATCH_RESERVATION"); err != nil {
				return err
			}
		}
		return nil
	})
	if err == nil {
		pkg.ID, pkg.Status, pkg.CreatedAt, pkg.SentAt = m.ID, domain.PackageStatusSending, m.CreatedAt, m.SentAt
		pkg.InvoiceIDs = slices.Clone(invoiceIDs)
	}
	return err
}

func (r *PostgresSentPackageRepository) UpdateBatch(pkg *domain.SentPackage, invoiceStatus *domain.InvoiceStatus) error {
	if pkg == nil || pkg.ID == "" {
		return fmt.Errorf("el lote requiere un identificador")
	}
	if invoiceStatus != nil && *invoiceStatus != domain.InvoiceSent && *invoiceStatus != domain.InvoiceAccepted &&
		*invoiceStatus != domain.InvoiceObserved && *invoiceStatus != domain.InvoiceRejected {
		return fmt.Errorf("estado de conciliación no permitido: %s", *invoiceStatus)
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		var current models.SentPackage
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ?", pkg.ID).First(&current).Error; err != nil {
			return err
		}
		if current.CompanyId != pkg.CompanyId || current.PointOfSaleId != pkg.PointOfSaleId {
			return fmt.Errorf("el lote pertenece a otro tenant o punto de venta")
		}
		if err := validateBatchSnapshot(current, pkg); err != nil {
			return err
		}
		var members []models.SentPackageInvoice
		if err := tx.Where("sent_package_id = ?", pkg.ID).Order("position").Find(&members).Error; err != nil {
			return err
		}
		ids := make([]string, len(members))
		for i, member := range members {
			ids[i] = member.InvoiceID
		}
		if len(pkg.InvoiceIDs) > 0 && !slices.Equal(ids, pkg.InvoiceIDs) {
			return fmt.Errorf("la pertenencia y orden de las facturas del lote son inmutables")
		}
		if len(pkg.Cufs) > 0 && len(pkg.Cufs) != len(ids) {
			return fmt.Errorf("cantidad de CUF distinta de la cantidad de facturas del lote")
		}
		if len(ids) > 0 && pkg.CantidadFacturas != len(ids) {
			return fmt.Errorf("cantidad de facturas inconsistente con la reserva")
		}
		if current.Status == string(domain.PackageStatusAccepted) || current.Status == string(domain.PackageStatusRejected) {
			if current.Status != string(pkg.Status) {
				return fmt.Errorf("el lote ya tiene un resultado fiscal definitivo")
			}
			return nil
		}
		if err := validateBatchStatus(domain.SentPackageStatus(current.Status), pkg.Status); err != nil {
			return err
		}
		updated := toModelSentPackage(pkg)
		if err := tx.Model(&models.SentPackage{}).Where("id = ?", pkg.ID).
			Select("codigo_recepcion", "hash_archivo", "status", "mensajes", "xml_hash", "validated_at").
			Updates(&updated).Error; err != nil {
			return err
		}
		if len(ids) == 0 || (invoiceStatus == nil && len(pkg.Cufs) == 0) {
			return nil
		}
		var invoices []models.Invoice
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id IN ?", ids).
			Order("id").Find(&invoices).Error; err != nil {
			return err
		}
		cufs := make(map[string]string, len(pkg.Cufs))
		for i, cuf := range pkg.Cufs {
			if cuf == "" {
				return fmt.Errorf("CUF vacío en respuesta del lote")
			}
			cufs[ids[i]] = cuf
		}
		if len(invoices) != len(ids) {
			return fmt.Errorf("faltan facturas de la reserva")
		}
		for _, invoice := range invoices {
			if cuf, ok := cufs[invoice.ID]; ok && invoice.Cuf != nil && *invoice.Cuf != "" && *invoice.Cuf != cuf {
				return fmt.Errorf("el CUF de la factura %s es inmutable", invoice.ID)
			}
			if invoice.Status != models.StatusSending && invoice.Status != models.StatusSent {
				continue // La conciliación individual puede haberse adelantado al lote.
			}
			values := map[string]any{}
			if invoiceStatus != nil {
				values["status"] = models.InvoiceStatus(*invoiceStatus)
			}
			if pkg.CodigoRecepcion != "" {
				values["siat_reception_code"] = pkg.CodigoRecepcion
			}
			if pkg.Mensajes != nil {
				values["siat_mensajes"] = pkg.Mensajes
			}
			if cuf, ok := cufs[invoice.ID]; ok {
				values["cuf"] = cuf
			}
			if pkg.Type == domain.PackageTypeMasiva {
				values["emission_type"] = models.EmissionMasiva
				if pkg.CufdID != "" {
					values["cufd_id"] = pkg.CufdID
				}
			}
			if len(values) > 0 {
				if err := tx.Model(&models.Invoice{}).Where("id = ? AND status IN ?", invoice.ID,
					[]models.InvoiceStatus{models.StatusSending, models.StatusSent}).Updates(values).Error; err != nil {
					return err
				}
			}
			if invoiceStatus != nil && domain.InvoiceStatus(invoice.Status) != *invoiceStatus {
				if err := recordBatchTransition(tx, invoice, pkg.ID, *invoiceStatus, "BATCH_RECONCILIATION"); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

// La consulta de un lote utiliza el contexto fiscal capturado en su reserva;
// ningún resultado tardío puede reescribir ese contexto ni su identidad.
func validateBatchSnapshot(current models.SentPackage, pkg *domain.SentPackage) error {
	if current.Type != string(pkg.Type) || current.CantidadFacturas != pkg.CantidadFacturas ||
		current.Modalidad != pkg.Modalidad || current.Layout != pkg.Layout || current.Cufd != pkg.Cufd ||
		current.Cuis != pkg.Cuis || !sameOptionalString(current.CufdID, toModelSentPackage(pkg).CufdID) ||
		current.CodigoDocumentoSector != pkg.CodigoDocumentoSector || current.CodigoTipoFactura != pkg.CodigoTipoFactura ||
		current.CodigoEmision != pkg.CodigoEmision || !sameOptionalString(current.ContingencyEventId, pkg.ContingencyEventId) ||
		!sameOptionalInt64(current.CodigoEvento, pkg.CodigoEvento) || current.SentAt.UnixMicro() != pkg.SentAt.UnixMicro() {
		return fmt.Errorf("los metadatos fiscales de la reserva son inmutables")
	}
	if current.CodigoRecepcion != "" && current.CodigoRecepcion != pkg.CodigoRecepcion {
		return fmt.Errorf("el código de recepción del lote es inmutable")
	}
	if current.HashArchivo != "" && current.HashArchivo != pkg.HashArchivo ||
		current.XmlHash != "" && current.XmlHash != pkg.XmlHash {
		return fmt.Errorf("los hashes del lote son inmutables")
	}
	return nil
}

func validateBatchStatus(from, to domain.SentPackageStatus) error {
	stages := map[domain.SentPackageStatus]int{
		domain.PackageStatusSending: 0, domain.PackageStatusUnknown: 1,
		domain.PackageStatusSent: 2, domain.PackageStatusPending: 3,
		domain.PackageStatusAccepted: 4, domain.PackageStatusRejected: 4,
	}
	stage, ok := stages[to]
	if !ok || stage < stages[from] {
		return fmt.Errorf("transición de lote no permitida: %s a %s", from, to)
	}
	return nil
}

func sameOptionalInt64(a, b *int64) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func sameOptionalString(a, b *string) bool {
	return (a == nil && b == nil) || (a != nil && b != nil && *a == *b)
}

func recordBatchTransition(tx *gorm.DB, invoice models.Invoice, packageID string, status domain.InvoiceStatus, reason string) error {
	payload, err := json.Marshal(map[string]string{
		"from_status": string(invoice.Status), "to_status": string(status),
		"reason": reason, "source": "FiscalBatchRepository", "sent_package_id": packageID,
	})
	if err != nil {
		return err
	}
	return tx.Create(&models.InvoiceEvent{
		ID: uuid.NewString(), InvoiceId: invoice.ID, TenantID: invoice.CompanyId,
		Type: "STATUS_TRANSITION", Message: fmt.Sprintf("invoice status changed from %s to %s", invoice.Status, status),
		Payload: datatypes.JSON(payload),
	}).Error
}
