package models

import "time"

type User struct {
	ID           string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Email        string `gorm:"type:varchar(320);not null"`
	Name         string `gorm:"type:varchar(150);not null"`
	PasswordHash string `gorm:"type:varchar(255);not null"`
	IsActive     bool   `gorm:"not null;default:true"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (User) TableName() string { return "users" }

type UserTenant struct {
	UserID    string    `gorm:"column:user_id;type:uuid;primaryKey"`
	TenantID  string    `gorm:"column:tenant_id;type:uuid;primaryKey"`
	Role      string    `gorm:"type:varchar(20);not null;default:'member'"`
	CreatedAt time.Time `gorm:"not null"`
}

func (UserTenant) TableName() string { return "user_tenants" }
