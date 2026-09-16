package domain

import "time"

// CertificateStatus indica el estado de un certificado digital.
type CertificateStatus string

const (
	CertificateActive  CertificateStatus = "ACTIVE"
	CertificateExpired CertificateStatus = "EXPIRED"
	CertificateRevoked CertificateStatus = "REVOKED"
	CertificatePending CertificateStatus = "PENDING"
)

// Certificate gestiona exclusivamente los certificados digitales (P12/PEM)
// usados para firmar facturas electrónicas ante el SIAT. El password y los
// bytes P12 se persisten cifrados con AES-GCM; nunca en texto plano.
type Certificate struct {
	ID           string            `json:"id"`
	CompanyId    string            `json:"company_id"`
	Name         string            `json:"name"`
	Type         string            `json:"type"` // P12, PEM
	Status       CertificateStatus `json:"status"`
	NotBefore    time.Time         `json:"not_before"`
	NotAfter     time.Time         `json:"not_after"`
	Issuer       string            `json:"issuer,omitempty"`
	Subject      string            `json:"subject,omitempty"`
	Thumbprint   string            `json:"thumbprint,omitempty"`
	SiatUserCode string            `json:"siat_user_code,omitempty"`
	ConfigPath   string            `json:"config_path,omitempty"`
	RenewedFrom  *string           `json:"renewed_from,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`

	// Material de firma cifrado en reposo.
	EncryptedP12Password string `json:"-"`
	P12StorageRef        string `json:"p12_storage_ref,omitempty"` // ref R2/local al .p12 cifrado

	Company Company `json:"company"`
}

// CertificateRepository define el contrato para la persistencia de certificados.
type CertificateRepository interface {
	Create(cert *Certificate) error
	GetByID(id string) (*Certificate, error)
	GetActiveByCompany(companyID string) (*Certificate, error)
	ListByCompany(companyID string) ([]*Certificate, error)
	Update(cert *Certificate) error
	Delete(id string) error
}
