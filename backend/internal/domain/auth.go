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
