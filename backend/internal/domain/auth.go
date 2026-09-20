package domain

import (
	"errors"
	"time"
)

const (
	CompanyRoleOwner  = "owner"
	CompanyRoleAdmin  = "admin"
	CompanyRoleMember = "member"
)

var ErrUserEmailConflict = errors.New("ya existe un usuario registrado con este correo")

// User representa la identidad humana que inicia sesión en el frontend. No está
// atada a una empresa: la relación multi-tenant vive en user_tenants.
type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	IsActive     bool      `json:"is_active"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UserCompany struct {
	Company *Company `json:"company"`
	Role    string   `json:"role"`
}

// AuthRepository agrupa la persistencia que necesita la autenticación humana.
// CreateCompanyForUser debe crear la empresa y la membresía owner de forma
// atómica para no dejar tenants huérfanos.
type AuthRepository interface {
	CreateUser(user *User) error
	GetUserByEmail(email string) (*User, error)
	GetUserByID(id string) (*User, error)
	HasCompanyAccess(userID, companyID string) (bool, error)
	ListCompanies(userID string) ([]UserCompany, error)
	CreateCompanyForUser(userID string, company *Company) error
}
