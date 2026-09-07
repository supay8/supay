package postgres

import (
	"encoding/json"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type PostgresCompanyRepository struct {
	db *gorm.DB
}

func NewPostgresCompanyRepository(db *gorm.DB) domain.CompanyRepository {
	return &PostgresCompanyRepository{db: db}
}

func (r *PostgresCompanyRepository) Create(c *domain.Company) error {
	modalidad := c.Modalidad
	if modalidad == 0 {
		modalidad = 1
	}
	tenant := models.Company{
		ID:              uuid.NewString(),
		Nit:             c.Nit,
		BusinessName:    c.BusinessName,
		Municipio:       c.Municipio,
		Direccion:       c.Direccion,
		Telefono:        c.Telefono,
		CodigoActividad: c.CodigoActividad,
		PiePagina:       c.PiePagina,
		UsuarioSiat:     c.UsuarioSiat,
	}
	settings, err := tenantSettingsWithCertificateWebhook(nil, c.CertificateWebhookURL)
	if err != nil {
		return err
	}
	config := models.TenantConfig{
		TenantID: tenant.ID,
		Ambiente: models.SiatEnvironment(c.Ambiente), CodigoModalidad: modalidad, Settings: settings,
	}

	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&tenant).Error; err != nil {
			return err
		}
		return tx.Create(&config).Error
	}); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCompanyNitConflict
		}
		return err
	}

	c.ID = tenant.ID
	c.CreatedAt = tenant.CreatedAt
	c.UpdatedAt = tenant.UpdatedAt
	return nil
}

func (r *PostgresCompanyRepository) GetByNit(nit string) (*domain.Company, error) {
	var tenant models.Company
	if err := r.db.Where("nit = ?", nit).First(&tenant).Error; err != nil {
		return nil, err
	}
	return r.withTenantConfig(&tenant)
}

func (r *PostgresCompanyRepository) GetByID(id string) (*domain.Company, error) {
	var tenant models.Company
	if err := r.db.Where("id = ?", id).First(&tenant).Error; err != nil {
		return nil, err
	}
	return r.withTenantConfig(&tenant)
}

func (r *PostgresCompanyRepository) withTenantConfig(tenant *models.Company) (*domain.Company, error) {
	var config models.TenantConfig
	if err := r.db.Where("tenant_id = ?", tenant.ID).First(&config).Error; err != nil {
		return nil, err
	}
	tenant.CodigoSistema = config.CodigoSistema
	tenant.Ambiente = config.Ambiente
	tenant.Modalidad = config.CodigoModalidad
	tenant.Config = config
	return toDomainCompany(tenant), nil
}

func toDomainCompany(dbModel *models.Company) *domain.Company {
	codigoSistema := dbModel.CodigoSistema
	ambiente := dbModel.Ambiente
	modalidad := dbModel.Modalidad
	if dbModel.Config.TenantID != "" {
		codigoSistema = dbModel.Config.CodigoSistema
		ambiente = dbModel.Config.Ambiente
		modalidad = dbModel.Config.CodigoModalidad
	}
	return &domain.Company{
		ID:                    dbModel.ID,
		Nit:                   dbModel.Nit,
		BusinessName:          dbModel.BusinessName,
		CodigoSistema:         codigoSistema,
		Ambiente:              domain.SiatEnvironment(ambiente),
		Modalidad:             modalidad,
		Municipio:             dbModel.Municipio,
		Direccion:             dbModel.Direccion,
		Telefono:              dbModel.Telefono,
		CodigoActividad:       dbModel.CodigoActividad,
		PiePagina:             dbModel.PiePagina,
		UsuarioSiat:           dbModel.UsuarioSiat,
		CertificateWebhookURL: certificateWebhookURL(json.RawMessage(dbModel.Config.Settings)),
		CreatedAt:             dbModel.CreatedAt,
		UpdatedAt:             dbModel.UpdatedAt,
	}
}

func (r *PostgresCompanyRepository) Update(c *domain.Company) error {
	var tenant models.Company
	if err := r.db.Where("id = ?", c.ID).First(&tenant).Error; err != nil {
		return err
	}
	var tenantConfig models.TenantConfig
	if err := r.db.Where("tenant_id = ?", c.ID).First(&tenantConfig).Error; err != nil {
		return err
	}

	tenant.Nit = c.Nit
	tenant.BusinessName = c.BusinessName
	modalidad := c.Modalidad
	if modalidad == 0 {
		modalidad = 1
	}
	tenant.Municipio = c.Municipio
	tenant.Direccion = c.Direccion
	tenant.Telefono = c.Telefono
	tenant.CodigoActividad = c.CodigoActividad
	tenant.PiePagina = c.PiePagina
	tenant.UsuarioSiat = c.UsuarioSiat

	settings, err := tenantSettingsWithCertificateWebhook(tenantConfig.Settings, c.CertificateWebhookURL)
	if err != nil {
		return err
	}
	if err := r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Save(&tenant).Error; err != nil {
			return err
		}
		return tx.Model(&models.TenantConfig{}).Where("tenant_id = ?", c.ID).Updates(map[string]any{
			"codigo_sistema":   c.CodigoSistema,
			"ambiente":         models.SiatEnvironment(c.Ambiente),
			"codigo_modalidad": modalidad,
			"settings":         settings,
			"updated_at":       gorm.Expr("now()"),
		}).Error
	}); err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCompanyNitConflict
		}
		return err
	}

	c.UpdatedAt = tenant.UpdatedAt
	return nil
}

func tenantSettingsWithCertificateWebhook(raw datatypes.JSON, webhookURL string) (datatypes.JSON, error) {
	settings := make(map[string]any)
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &settings); err != nil {
			return nil, err
		}
	}
	// Normaliza el formato legacy top-level al bloque canónico.
	delete(settings, "certificate_webhook_url")
	alerts, _ := settings["certificate_alerts"].(map[string]any)
	if alerts == nil {
		alerts = make(map[string]any)
	}
	if webhookURL == "" {
		delete(alerts, "webhook_url")
	} else {
		alerts["webhook_url"] = webhookURL
	}
	if len(alerts) == 0 {
		delete(settings, "certificate_alerts")
	} else {
		settings["certificate_alerts"] = alerts
	}
	encoded, err := json.Marshal(settings)
	return datatypes.JSON(encoded), err
}

func (r *PostgresCompanyRepository) Delete(id string) error {
	if err := r.db.Where("id = ?", id).Delete(&models.Company{}).Error; err != nil {
		if isForeignKeyViolation(err) {
			return domain.ErrCompanyHasDependencies
		}
		return err
	}
	return nil
}
