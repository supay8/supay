package postgres

import (
	"errors"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PostgresAuthRepository struct {
	db *gorm.DB
}

func NewPostgresAuthRepository(db *gorm.DB) *PostgresAuthRepository {
	return &PostgresAuthRepository{db: db}
}

func (r *PostgresAuthRepository) CreateUser(user *domain.User) error {
	model := models.User{
		ID:           uuid.NewString(),
		Email:        strings.ToLower(strings.TrimSpace(user.Email)),
		Name:         strings.TrimSpace(user.Name),
		PasswordHash: user.PasswordHash,
		IsActive:     true,
	}
	if err := r.db.Create(&model).Error; err != nil {
		if isUniqueViolation(err) {
			return domain.ErrUserEmailConflict
		}
		return err
	}
	*user = *toDomainUser(&model)
	return nil
}

func (r *PostgresAuthRepository) GetUserByEmail(email string) (*domain.User, error) {
	var user models.User
	if err := r.db.Where("lower(email) = ?", strings.ToLower(strings.TrimSpace(email))).First(&user).Error; err != nil {
		return nil, err
	}
	return toDomainUser(&user), nil
}

func (r *PostgresAuthRepository) GetUserByID(id string) (*domain.User, error) {
	var user models.User
	if err := r.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return toDomainUser(&user), nil
}

func (r *PostgresAuthRepository) HasCompanyAccess(userID, companyID string) (bool, error) {
	var count int64
	err := r.db.Table("user_tenants AS ut").
		Joins("JOIN users AS u ON u.id = ut.user_id AND u.is_active = true").
		Joins("JOIN tenants AS t ON t.id = ut.tenant_id AND t.is_active = true").
		Where("ut.user_id = ? AND ut.tenant_id = ?", userID, companyID).
		Count(&count).Error
	return count > 0, err
}

func (r *PostgresAuthRepository) ListCompanies(userID string) ([]domain.UserCompany, error) {
	var memberships []models.UserTenant
	if err := r.db.Where("user_id = ?", userID).Order("created_at ASC").Find(&memberships).Error; err != nil {
		return nil, err
	}
	result := make([]domain.UserCompany, 0, len(memberships))
	companyRepo := &PostgresCompanyRepository{db: r.db}
	for _, membership := range memberships {
		company, err := companyRepo.GetByID(membership.TenantID)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		result = append(result, domain.UserCompany{Company: company, Role: membership.Role})
	}
	return result, nil
}

func (r *PostgresAuthRepository) CreateCompanyForUser(userID string, company *domain.Company) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := createCompany(tx, company); err != nil {
			if isUniqueViolation(err) {
				return domain.ErrCompanyNitConflict
			}
			return err
		}
		membership := models.UserTenant{
			UserID:   userID,
			TenantID: company.ID,
			Role:     domain.CompanyRoleOwner,
		}
		return tx.Create(&membership).Error
	})
}

func toDomainUser(user *models.User) *domain.User {
	return &domain.User{
		ID:           user.ID,
		Email:        user.Email,
		Name:         user.Name,
		PasswordHash: user.PasswordHash,
		IsActive:     user.IsActive,
		CreatedAt:    user.CreatedAt,
		UpdatedAt:    user.UpdatedAt,
	}
}
