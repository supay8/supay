package models

import "time"

// ApiKey representa una clave de API perteneciente a un tenant/empresa.
// El valor plano de la key nunca se almacena; solo se guarda su hash
// (recomendado bcrypt/argon2) y un prefijo identificador.
type ApiKey struct {
	ID         string `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CompanyId  string `gorm:"column:tenant_id;type:uuid;index:idx_api_keys_tenant;not null"`
	KeyHash    string `gorm:"type:varchar(255);uniqueIndex;not null"`
	KeyPrefix  string `gorm:"type:varchar(20);not null"`
	Name       string `gorm:"type:varchar(100);not null;default:'default'"`
	Scopes     string `gorm:"type:varchar(255);not null;default:'read,write'"`
	IsActive   bool   `gorm:"default:true;not null"`
	LastUsedAt *time.Time
	ExpiresAt  *time.Time
	CreatedAt  time.Time
	UpdatedAt  time.Time

	Company Company `gorm:"foreignKey:CompanyId"`
}

func (ApiKey) TableName() string { return "api_keys" }
