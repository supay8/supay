package domain

import "time"

type ApiKey struct {
	ID, CompanyId, KeyHash, KeyPrefix, Name string
	Scopes                                  []string
	IsActive                                bool
	LastUsedAt, ExpiresAt                   *time.Time
	CreatedAt, UpdatedAt                    time.Time
}
