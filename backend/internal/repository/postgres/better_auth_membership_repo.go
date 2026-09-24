package postgres

import "gorm.io/gorm"

// BetterAuthMembershipRepository autoriza tenants cloud usando la membresía
// canónica del plugin Organizations. El header X-Company-ID sigue refiriéndose
// al tenant fiscal de Supay; auth_organization_id realiza el enlace 1:1.
type BetterAuthMembershipRepository struct {
	db *gorm.DB
}

func NewBetterAuthMembershipRepository(db *gorm.DB) *BetterAuthMembershipRepository {
	return &BetterAuthMembershipRepository{db: db}
}

func (r *BetterAuthMembershipRepository) HasCompanyAccess(userID, companyID string) (bool, error) {
	var count int64
	err := r.db.Table("auth.members AS member").
		Joins("JOIN tenants AS tenant ON tenant.auth_organization_id = member.organization_id AND tenant.is_active = true").
		Where("member.user_id = ? AND tenant.id = ?", userID, companyID).
		Count(&count).Error
	return count > 0, err
}
