package postgres

import (
	"errors"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
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
		// Advisory lock por punto de venta: serializa la asignación del
		// número correlativo entre emisiones concurrentes.
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?), hashtext(?))",
			"invoice_number", inv.PointOfSaleId).Error; err != nil {
			return err
		}
		var maxNumber int
		if err := tx.Raw(
			"SELECT COALESCE(MAX(invoice_number), 0) FROM invoices WHERE point_of_sale_id = ?",
			inv.PointOfSaleId).Scan(&maxNumber).Error; err != nil {
			return err
		}
		inv.InvoiceNumber = maxNumber + 1

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

func (r *PostgresInvoiceRepository) GetByID(id string) (*domain.Invoice, error) {
	var m models.Invoice
	if err := r.db.Preload("Items").Preload("PointOfSale").Preload("Company").Preload("Customer").Preload("CufdRecord").First(&m, "id = ?", id).Error; err != nil {
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

func (r *PostgresInvoiceRepository) Update(inv *domain.Invoice) error {
	m := toModelInvoice(inv)
	return r.db.Save(&m).Error
}

func (r *PostgresInvoiceRepository) ClaimForEmission(id string) (bool, error) {
	res := r.db.Model(&models.Invoice{}).
		Where("id = ? AND status = ?", id, models.StatusPending).
		Update("status", models.StatusSending)
	if res.Error != nil {
		return false, res.Error
	}
	return res.RowsAffected == 1, nil
}

func (r *PostgresInvoiceRepository) FindActiveCufdForPointOfSale(pointOfSaleID string, at time.Time) (*domain.Cufd, error) {
	var cufd models.Cufd
	if err := r.db.Where("point_of_sale_id = ? AND valid_from <= ? AND valid_to >= ?", pointOfSaleID, at, at).Order("valid_from desc").First(&cufd).Error; err != nil {
		return nil, err
	}
	return toDomainCufd(&cufd), nil
}

func toModelInvoice(inv *domain.Invoice) models.Invoice {
	m := models.Invoice{
		ID:                inv.ID,
		CompanyId:         inv.CompanyId,
		CustomerId:        inv.CustomerId,
		PointOfSaleId:     inv.PointOfSaleId,
		CufdId:            inv.CufdId,
		InvoiceNumber:     inv.InvoiceNumber,
		Cuf:               inv.Cuf,
		EmissionType:      models.EmissionType(inv.EmissionType),
		CodigoMetodoPago:  inv.CodigoMetodoPago,
		CodigoMoneda:      inv.CodigoMoneda,
		TipoCambio:        inv.TipoCambio,
		CodigoDocumentoSector: inv.CodigoDocumentoSector,
		CodigoTipoFactura: inv.CodigoTipoFactura,
		NombreEstudiante:  inv.NombreEstudiante,
		PeriodoFacturado:  inv.PeriodoFacturado,
		IssueDate:         inv.IssueDate,
		Subtotal:          inv.Subtotal,
		Discount:          inv.Discount,
		Total:             inv.Total,
		Xml:               inv.Xml,
		XmlHash:           inv.XmlHash,
		SiatReceptionCode: inv.SiatReceptionCode,
		SiatMensajes:      inv.SiatMensajes,
		MotivoAnulacion:   inv.MotivoAnulacion,
		FechaAnulacion:    inv.FechaAnulacion,
		Status:            models.InvoiceStatus(inv.Status),
		CreatedAt:         inv.CreatedAt,
	}
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	for i := range inv.Items {
		item := inv.Items[i]
		mi := models.InvoiceItem{
			ID:                item.ID,
			InvoiceId:         item.InvoiceId,
			Code:              item.Code,
			Description:       item.Description,
			CodigoActividad:   item.CodigoActividad,
			CodigoProductoSin: item.CodigoProductoSin,
			UnitCode:          item.UnitCode,
			Quantity:          item.Quantity,
			UnitPrice:         item.UnitPrice,
			Discount:          item.Discount,
			Subtotal:          item.Subtotal,
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
		ID:                m.ID,
		CompanyId:         m.CompanyId,
		CustomerId:        m.CustomerId,
		PointOfSaleId:     m.PointOfSaleId,
		CufdId:            m.CufdId,
		InvoiceNumber:     m.InvoiceNumber,
		Cuf:               m.Cuf,
		EmissionType:      string(m.EmissionType),
		CodigoMetodoPago:  m.CodigoMetodoPago,
		CodigoMoneda:      m.CodigoMoneda,
		TipoCambio:        m.TipoCambio,
		CodigoDocumentoSector: m.CodigoDocumentoSector,
		CodigoTipoFactura: m.CodigoTipoFactura,
		NombreEstudiante:  m.NombreEstudiante,
		PeriodoFacturado:  m.PeriodoFacturado,
		IssueDate:         m.IssueDate,
		Subtotal:          m.Subtotal,
		Discount:          m.Discount,
		Total:             m.Total,
		Xml:               m.Xml,
		XmlHash:           m.XmlHash,
		SiatReceptionCode: m.SiatReceptionCode,
		SiatMensajes:      m.SiatMensajes,
		MotivoAnulacion:   m.MotivoAnulacion,
		FechaAnulacion:    m.FechaAnulacion,
		Status:            domain.InvoiceStatus(m.Status),
		CreatedAt:         m.CreatedAt,
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
			Code:              mi.Code,
			Description:       mi.Description,
			CodigoActividad:   mi.CodigoActividad,
			CodigoProductoSin: mi.CodigoProductoSin,
			UnitCode:          mi.UnitCode,
			Quantity:          mi.Quantity,
			UnitPrice:         mi.UnitPrice,
			Discount:          mi.Discount,
			Subtotal:          mi.Subtotal,
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

func isNotFound(err error) bool {
	return err != nil && errors.Is(err, gorm.ErrRecordNotFound)
}
