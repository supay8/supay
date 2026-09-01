package postgres

import (
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
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
	dbModel := models.Company{
		ID:              uuid.NewString(),
		Nit:             c.Nit,
		BusinessName:    c.BusinessName,
		CodigoSistema:   c.CodigoSistema,
		Ambiente:        models.SiatEnvironment(c.Ambiente),
		Modalidad:       modalidad,
		Municipio:       c.Municipio,
		Direccion:       c.Direccion,
		Telefono:        c.Telefono,
		CodigoActividad: c.CodigoActividad,
		PiePagina:       c.PiePagina,
		UsuarioSiat:     c.UsuarioSiat,
	}

	if err := r.db.Create(&dbModel).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCompanyNitConflict
		}
		return err
	}

	c.ID = dbModel.ID
	c.CreatedAt = dbModel.CreatedAt
	c.UpdatedAt = dbModel.UpdatedAt
	return nil
}

func (r *PostgresCompanyRepository) GetByNit(nit string) (*domain.Company, error) {
	var dbModel models.Company
	if err := r.db.Where("nit = ?", nit).First(&dbModel).Error; err != nil {
		return nil, err
	}
	return toDomainCompany(&dbModel), nil
}

func (r *PostgresCompanyRepository) GetByID(id string) (*domain.Company, error) {
	var dbModel models.Company
	if err := r.db.Where("id = ?", id).First(&dbModel).Error; err != nil {
		return nil, err
	}
	return toDomainCompany(&dbModel), nil
}

func toDomainCompany(dbModel *models.Company) *domain.Company {
	return &domain.Company{
		ID:              dbModel.ID,
		Nit:             dbModel.Nit,
		BusinessName:    dbModel.BusinessName,
		CodigoSistema:   dbModel.CodigoSistema,
		Ambiente:        domain.SiatEnvironment(dbModel.Ambiente),
		Modalidad:       dbModel.Modalidad,
		Municipio:       dbModel.Municipio,
		Direccion:       dbModel.Direccion,
		Telefono:        dbModel.Telefono,
		CodigoActividad: dbModel.CodigoActividad,
		PiePagina:       dbModel.PiePagina,
		UsuarioSiat:     dbModel.UsuarioSiat,
		CreatedAt:       dbModel.CreatedAt,
		UpdatedAt:       dbModel.UpdatedAt,
	}
}

func (r *PostgresCompanyRepository) Update(c *domain.Company) error {
	var dbModel models.Company
	if err := r.db.Where("id = ?", c.ID).First(&dbModel).Error; err != nil {
		return err
	}

	dbModel.Nit = c.Nit
	dbModel.BusinessName = c.BusinessName
	dbModel.CodigoSistema = c.CodigoSistema
	dbModel.Ambiente = models.SiatEnvironment(c.Ambiente)
	dbModel.Modalidad = c.Modalidad
	if dbModel.Modalidad == 0 {
		dbModel.Modalidad = 1
	}
	dbModel.Municipio = c.Municipio
	dbModel.Direccion = c.Direccion
	dbModel.Telefono = c.Telefono
	dbModel.CodigoActividad = c.CodigoActividad
	dbModel.PiePagina = c.PiePagina
	dbModel.UsuarioSiat = c.UsuarioSiat

	if err := r.db.Save(&dbModel).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrCompanyNitConflict
		}
		return err
	}

	c.UpdatedAt = dbModel.UpdatedAt
	return nil
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
