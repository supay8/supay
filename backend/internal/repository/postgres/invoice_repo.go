package postgres

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type PostgresInvoiceRepository struct {
	db *gorm.DB
}

func NewPostgresInvoiceRepository(db *gorm.DB) domain.InvoiceRepository {
	return &PostgresInvoiceRepository{db: db}
}

func (r *PostgresInvoiceRepository) Create(inv *domain.Invoice) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		// La secuencia se incrementa atómicamente y evita escanear invoices.
		if err := tx.Exec(`
			INSERT INTO invoice_sequences (tenant_id, point_of_sale_id, next_number)
			VALUES (?, ?, 1)
			ON CONFLICT (tenant_id, point_of_sale_id) DO NOTHING`,
			inv.CompanyId, inv.PointOfSaleId).Error; err != nil {
			return err
		}
		if err := tx.Raw(`
			UPDATE invoice_sequences
			SET next_number = next_number + 1, updated_at = now()
			WHERE tenant_id = ? AND point_of_sale_id = ?
			RETURNING next_number - 1`, inv.CompanyId, inv.PointOfSaleId).
			Scan(&inv.InvoiceNumber).Error; err != nil {
			return err
		}

		m := toModelInvoice(inv)
		if err := tx.Create(&m).Error; err != nil {
			return err
		}
		inv.ID = m.ID
		inv.CreatedAt = m.CreatedAt
		for i := range inv.Items {
			if i < len(m.Items) {
				inv.Items[i].ID = m.Items[i].ID
				inv.Items[i].InvoiceId = m.Items[i].InvoiceId
			}
		}
		return nil
	})
}

// ListFiltered devuelve facturas paginadas según el filtro omitiendo los
// campos pesados (xml, archivo) y pre-cargando ítems y cliente.
func (r *PostgresInvoiceRepository) ListFiltered(filter domain.InvoiceListFilter) ([]*domain.Invoice, int64, error) {
	query := r.db.Model(&models.Invoice{}).Where("point_of_sale_id = ?", filter.PointOfSaleID)
	if filter.Status != nil {
		query = query.Where("status = ?", string(*filter.Status))
	}
	if filter.From != nil {
		query = query.Where("issue_date >= ?", *filter.From)
	}
	if filter.To != nil {
		query = query.Where("issue_date <= ?", *filter.To)
	}
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var ms []models.Invoice
	if err := query.
		Omit("xml", "archivo").
		Preload("Items").Preload("Customer").
		Order("invoice_number DESC").
		Limit(filter.Limit).Offset(filter.Offset).
		Find(&ms).Error; err != nil {
		return nil, 0, err
	}
	res := make([]*domain.Invoice, 0, len(ms))
	for i := range ms {
		res = append(res, toDomainInvoice(&ms[i]))
	}
	return res, total, nil
}

func (r *PostgresInvoiceRepository) GetByID(id string) (*domain.Invoice, error) {
	var m models.Invoice
	if err := r.db.Preload("Items").Preload("PointOfSale").Preload("Company.Config").Preload("Customer").Preload("CufdRecord").First(&m, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return toDomainInvoice(&m), nil
}

func (r *PostgresInvoiceRepository) ListByPointOfSale(pointOfSaleID string) ([]*domain.Invoice, error) {
	var ms []models.Invoice
	if err := r.db.Preload("Items").Preload("Customer").
		Where("point_of_sale_id = ?", pointOfSaleID).
		Order("invoice_number ASC").Find(&ms).Error; err != nil {
		return nil, err
	}
	res := make([]*domain.Invoice, 0, len(ms))
	for i := range ms {
		res = append(res, toDomainInvoice(&ms[i]))
	}
	return res, nil
}

// invoiceMutableFields son las únicas columnas que la lógica de negocio muta
// después de crear la factura. Update() es parcial sobre estas columnas para
// que una escritura con datos obsoletos no pise campos no relacionados
// (montos, número correlativo, etc.).
func invoiceMutableFields(inv *domain.Invoice) map[string]any {
	return map[string]any{
		"cuf":                 inv.Cuf,
		"xml":                 inv.Xml,
		"xml_hash":            inv.XmlHash,
		"archivo":             inv.Archivo,
		"hash_archivo":        inv.HashArchivo,
		"siat_reception_code": inv.SiatReceptionCode,
		"siat_mensajes":       inv.SiatMensajes,
		"motivo_anulacion":    inv.MotivoAnulacion,
		"fecha_anulacion":     inv.FechaAnulacion,
	}
}

func (r *PostgresInvoiceRepository) Update(inv *domain.Invoice) error {
	var current models.Invoice
	if err := r.db.Select("status").Where("id = ?", inv.ID).First(&current).Error; err != nil {
		return err
	}
	if domain.InvoiceStatus(current.Status) != inv.Status {
		return domain.ErrStatusUpdateRequiresTransition
	}
	res := r.db.Model(&models.Invoice{}).
		Where("id = ?", inv.ID).
		Updates(invoiceMutableFields(inv))
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		var count int64
		if err := r.db.Model(&models.Invoice{}).Where("id = ?", inv.ID).Count(&count).Error; err != nil {
			return err
		}
		if count == 0 {
			return gorm.ErrRecordNotFound
		}
	}
	return nil
}

func (r *PostgresInvoiceRepository) TransitionStatus(id string, from, to domain.InvoiceStatus, reason domain.InvoiceTransitionReason, fields map[string]any, event *domain.InvoiceEvent) (bool, error) {
	if err := (domain.InvoiceStateMachine{}).Transition(from, to, reason); err != nil {
		return false, err
	}
	claimed := false
	err := r.db.Transaction(func(tx *gorm.DB) error {
		var tenantID string
		if err := tx.Model(&models.Invoice{}).Select("tenant_id").Where("id = ?", id).Scan(&tenantID).Error; err != nil {
			return err
		}
		if tenantID == "" {
			return gorm.ErrRecordNotFound
		}
		values := make(map[string]any, len(fields)+1)
		for key, value := range fields {
			if key != "status" {
				values[key] = value
			}
		}
		values["status"] = models.InvoiceStatus(to)
		result := tx.Model(&models.Invoice{}).
			Where("id = ? AND status = ?", id, models.InvoiceStatus(from)).
			Updates(values)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return nil
		}
		claimed = true
		if err := persistInvoiceDocuments(tx, id, fields); err != nil {
			return err
		}
		if event == nil {
			return nil
		}
		if event.InvoiceID == "" {
			event.InvoiceID = id
		}
		if event.TenantID == "" {
			event.TenantID = tenantID
		}
		model := models.InvoiceEvent{
			ID: event.ID, InvoiceId: event.InvoiceID, TenantID: event.TenantID,
			EventKey: event.EventKey, Type: event.Type, Message: event.Message,
			Payload: datatypes.JSON(event.Payload),
		}
		if model.ID == "" {
			model.ID = uuid.NewString()
		}
		if err := tx.Create(&model).Error; err != nil {
			return err
		}
		event.ID = model.ID
		event.CreatedAt = model.CreatedAt
		return nil
	})
	return claimed, err
}

func persistInvoiceDocuments(tx *gorm.DB, invoiceID string, fields map[string]any) error {
	definitions := []struct {
		key, hashKey, documentType, mimeType string
	}{
		{"xml", "xml_hash", "XML", "application/xml"},
		{"archivo", "hash_archivo", "FILE", "application/octet-stream"},
	}
	for _, definition := range definitions {
		content, ok := fields[definition.key].(*string)
		if !ok {
			if value, stringOK := fields[definition.key].(string); stringOK && strings.TrimSpace(value) != "" {
				content = &value
				ok = true
			}
		}
		if !ok || content == nil || strings.TrimSpace(*content) == "" {
			continue
		}
		var hash *string
		if value, ok := fields[definition.hashKey].(*string); ok {
			hash = value
		} else if value, ok := fields[definition.hashKey].(string); ok && value != "" {
			hash = &value
		}
		document := &domain.InvoiceDocument{
			InvoiceID: invoiceID, DocumentType: domain.InvoiceDocumentType(definition.documentType),
			Content: content, SHA256: hash, MIMEType: &definition.mimeType, IsCurrent: true,
		}
		if _, err := rotateInvoiceDocument(tx, document); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresInvoiceRepository) ClaimForEmission(id string) (bool, error) {
	if err := (domain.InvoiceStateMachine{}).Transition(domain.InvoicePending, domain.InvoiceSending, domain.TransitionEmissionStart); err != nil {
		return false, err
	}
	res := r.db.Model(&models.Invoice{}).
		Where("id = ? AND status = ?", id, models.StatusPending).
		Update("status", models.StatusSending)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *PostgresInvoiceRepository) ReleaseStaleSending(olderThan time.Duration) (int64, error) {
	cutoff := time.Now().Add(-olderThan)
	// updated_at IS NULL cubre filas históricas previas a la columna.
	res := r.db.Model(&models.Invoice{}).
		Where("status = ? AND (updated_at IS NULL OR updated_at < ?)", models.StatusSending, cutoff).
		Update("status", models.StatusPending)
	if res.Error != nil {
		return 0, res.Error
	}
	return res.RowsAffected, nil
}

func (r *PostgresInvoiceRepository) GetByIdempotencyKey(pointOfSaleID, key string) (*domain.Invoice, error) {
	var m models.Invoice
	if err := r.db.Preload("Items").Preload("PointOfSale").Preload("Company.Config").Preload("Customer").Preload("CufdRecord").
		First(&m, "point_of_sale_id = ? AND idempotency_key = ?", pointOfSaleID, key).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return toDomainInvoice(&m), nil
}

func (r *PostgresInvoiceRepository) FindActiveCufdForPointOfSale(pointOfSaleID string, at time.Time) (*domain.Cufd, error) {
	var cufd models.Cufd
	if err := r.db.Where("point_of_sale_id = ? AND valid_from <= ? AND valid_to >= ?", pointOfSaleID, at, at).Order("valid_from desc").First(&cufd).Error; err != nil {
		return nil, err
	}
	return toDomainCufd(&cufd), nil
}

// get list of invoices by  IDS
func (r *PostgresInvoiceRepository) GetByIDs(ids []string) ([]*domain.Invoice, error) {
	var ms []models.Invoice
	if err := r.db.Preload("Items").Preload("Customer").Preload("CufdRecord").
		Where("id IN ?", ids).
		Order("invoice_number ASC").Find(&ms).Error; err != nil {
		return nil, err
	}
	res := make([]*domain.Invoice, 0, len(ms))
	for i := range ms {
		res = append(res, toDomainInvoice(&ms[i]))
	}
	return res, nil

}

func toModelInvoice(inv *domain.Invoice) models.Invoice {
	m := models.Invoice{
		ID:                    inv.ID,
		CompanyId:             inv.CompanyId,
		CustomerId:            inv.CustomerId,
		PointOfSaleId:         inv.PointOfSaleId,
		IdempotencyKey:        inv.IdempotencyKey,
		CufdId:                inv.CufdId,
		ContingencyEventId:    inv.ContingencyEventId,
		InvoiceNumber:         inv.InvoiceNumber,
		Cuf:                   inv.Cuf,
		EmissionType:          models.EmissionType(inv.EmissionType),
		CodigoMetodoPago:      inv.CodigoMetodoPago,
		CodigoMoneda:          inv.CodigoMoneda,
		TipoCambio:            inv.TipoCambio,
		CodigoDocumentoSector: inv.CodigoDocumentoSector,
		Layout:                inv.Layout,
		Modalidad:             inv.Modalidad,
		CodigoTipoFactura:     inv.CodigoTipoFactura,
		Archivo:               inv.Archivo,
		HashArchivo:           inv.HashArchivo,
		NombreEstudiante:      inv.NombreEstudiante,
		PeriodoFacturado:      inv.PeriodoFacturado,
		SectorData:            datatypes.JSON(inv.SectorData),
		AjustaFacturaId:       inv.AjustaFacturaId,
		IssueDate:             inv.IssueDate,
		Subtotal:              inv.Subtotal,
		Discount:              inv.Discount,
		Total:                 inv.Total,
		Xml:                   inv.Xml,
		XmlHash:               inv.XmlHash,
		SiatReceptionCode:     inv.SiatReceptionCode,
		SiatMensajes:          inv.SiatMensajes,
		MotivoAnulacion:       inv.MotivoAnulacion,
		FechaAnulacion:        inv.FechaAnulacion,
		Status:                models.InvoiceStatus(inv.Status),
		CreatedAt:             inv.CreatedAt,
	}
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	for i := range inv.Items {
		item := inv.Items[i]
		mi := models.InvoiceItem{
			ID:                item.ID,
			InvoiceId:         item.InvoiceId,
			ProductId:         item.ProductID,
			Code:              item.Code,
			Description:       item.Description,
			CodigoActividad:   item.CodigoActividad,
			CodigoProductoSin: item.CodigoProductoSin,
			UnitCode:          item.UnitCode,
			Quantity:          item.Quantity,
			UnitPrice:         item.UnitPrice,
			Discount:          item.Discount,
			Subtotal:          item.Subtotal,
			SectorData:        datatypes.JSON(item.SectorData),
		}
		if mi.ID == "" {
			mi.ID = uuid.NewString()
		}
		m.Items = append(m.Items, mi)
	}
	return m
}

func toDomainInvoice(m *models.Invoice) *domain.Invoice {
	inv := &domain.Invoice{
		ID:                    m.ID,
		CompanyId:             m.CompanyId,
		CustomerId:            m.CustomerId,
		PointOfSaleId:         m.PointOfSaleId,
		IdempotencyKey:        m.IdempotencyKey,
		CufdId:                m.CufdId,
		ContingencyEventId:    m.ContingencyEventId,
		InvoiceNumber:         m.InvoiceNumber,
		Cuf:                   m.Cuf,
		EmissionType:          string(m.EmissionType),
		CodigoMetodoPago:      m.CodigoMetodoPago,
		CodigoMoneda:          m.CodigoMoneda,
		TipoCambio:            m.TipoCambio,
		CodigoDocumentoSector: m.CodigoDocumentoSector,
		Layout:                m.Layout,
		Modalidad:             m.Modalidad,
		CodigoTipoFactura:     m.CodigoTipoFactura,
		Archivo:               m.Archivo,
		HashArchivo:           m.HashArchivo,
		NombreEstudiante:      m.NombreEstudiante,
		PeriodoFacturado:      m.PeriodoFacturado,
		SectorData:            json.RawMessage(m.SectorData),
		AjustaFacturaId:       m.AjustaFacturaId,
		IssueDate:             m.IssueDate,
		Subtotal:              m.Subtotal,
		Discount:              m.Discount,
		Total:                 m.Total,
		Xml:                   m.Xml,
		XmlHash:               m.XmlHash,
		SiatReceptionCode:     m.SiatReceptionCode,
		SiatMensajes:          m.SiatMensajes,
		MotivoAnulacion:       m.MotivoAnulacion,
		FechaAnulacion:        m.FechaAnulacion,
		Status:                domain.InvoiceStatus(m.Status),
		CreatedAt:             m.CreatedAt,
	}
	inv.Company = *toDomainCompany(&m.Company)
	inv.Customer = *toDomainCustomer(&m.Customer)
	inv.PointOfSale = *toDomainPointOfSale(&m.PointOfSale)
	inv.CufdRecord = *toDomainCufd(&m.CufdRecord)
	inv.Items = make([]domain.InvoiceItem, 0, len(m.Items))
	for i := range m.Items {
		mi := m.Items[i]
		inv.Items = append(inv.Items, domain.InvoiceItem{
			ID:                mi.ID,
			InvoiceId:         mi.InvoiceId,
			ProductID:         mi.ProductId,
			Code:              mi.Code,
			Description:       mi.Description,
			CodigoActividad:   mi.CodigoActividad,
			CodigoProductoSin: mi.CodigoProductoSin,
			UnitCode:          mi.UnitCode,
			Quantity:          mi.Quantity,
			UnitPrice:         mi.UnitPrice,
			Discount:          mi.Discount,
			Subtotal:          mi.Subtotal,
			SectorData:        json.RawMessage(mi.SectorData),
		})
	}
	return inv
}

func toDomainCufd(m *models.Cufd) *domain.Cufd {
	return &domain.Cufd{
		ID:            m.ID,
		PointOfSaleID: m.PointOfSaleId,
		Cufd:          m.Cufd,
		ControlCode:   m.CodigoControl,
		Direccion:     m.Direccion,
		CodigoQR:      m.CodigoQR,
		ValidFrom:     m.ValidFrom,
		ValidTo:       m.ValidTo,
		Active:        m.Active,
		CreatedAt:     m.CreatedAt,
	}
}
