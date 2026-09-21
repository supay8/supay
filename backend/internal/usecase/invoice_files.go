package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"time"

	"github.com/brandsrx/supay/internal/domain"
)

// InvoiceFileService owns fiscal key construction, tenant checks and the
// object-before-database compensation policy.
type InvoiceFileService struct {
	storage    domain.Storage
	repo       domain.InvoiceFileRepository
	presignTTL time.Duration
}

func NewInvoiceFileService(storage domain.Storage, repo domain.InvoiceFileRepository, ttl time.Duration) *InvoiceFileService {
	return &InvoiceFileService{storage: storage, repo: repo, presignTTL: ttl}
}

func (s *InvoiceFileService) Save(ctx context.Context, companyID, invoiceID, cuf, kind string, reader io.Reader, size int64) (*domain.InvoiceFile, error) {
	key, err := domain.InvoiceObjectKey(companyID, invoiceID, cuf, kind)
	if err != nil {
		return nil, err
	}
	belongs, err := s.repo.BelongsToCompany(ctx, companyID, invoiceID)
	if err != nil {
		return nil, err
	}
	if !belongs {
		return nil, domain.ErrNotFound
	}
	contentType := "application/xml"
	if kind == "pdf" {
		contentType = "application/pdf"
	}
	hash := sha256.New()
	info, err := s.storage.Put(ctx, key, io.TeeReader(reader, hash), domain.PutOptions{ContentType: contentType, Size: size})
	if errors.Is(err, domain.ErrAlreadyExists) {
		if _, hashErr := io.Copy(hash, reader); hashErr != nil {
			return nil, hashErr
		}
		existing, findErr := s.repo.FindFile(ctx, companyID, invoiceID, kind)
		if findErr == nil && existing.StorageKey == key && existing.SHA256 == hex.EncodeToString(hash.Sum(nil)) {
			return existing, nil
		}
		return nil, fmt.Errorf("invoice object exists without matching metadata; reconcile %s: %w", key, domain.ErrAlreadyExists)
	}
	if err != nil {
		return nil, err
	}
	file := &domain.InvoiceFile{CompanyID: companyID, InvoiceID: invoiceID, Kind: kind, StorageKey: key, SHA256: info.SHA256, Size: info.Size, ContentType: contentType, CreatedAt: info.CreatedAt}
	if err := s.repo.CreateFile(ctx, file); err != nil {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if deleteErr := s.storage.Delete(cleanupCtx, key); deleteErr != nil {
			slog.Error("invoice file metadata failed; object cleanup failed; reconcile", "key", key, "invoice_id", invoiceID, "error", err, "cleanup_error", deleteErr)
		} else {
			slog.Error("invoice file metadata failed; object deleted", "key", key, "invoice_id", invoiceID, "error", err)
		}
		return nil, fmt.Errorf("invoice file metadata: %w", err)
	}
	return file, nil
}

func (s *InvoiceFileService) Open(ctx context.Context, companyID, invoiceID, kind string) (io.ReadCloser, domain.ObjectInfo, error) {
	file, err := s.file(ctx, companyID, invoiceID, kind)
	if err != nil {
		return nil, domain.ObjectInfo{}, err
	}
	return s.storage.Get(ctx, file.StorageKey)
}

func (s *InvoiceFileService) Presign(ctx context.Context, companyID, invoiceID, kind, filename string) (string, error) {
	file, err := s.file(ctx, companyID, invoiceID, kind)
	if err != nil {
		return "", err
	}
	return s.storage.PresignGet(ctx, file.StorageKey, s.presignTTL, filename)
}

func (s *InvoiceFileService) file(ctx context.Context, companyID, invoiceID, kind string) (*domain.InvoiceFile, error) {
	if kind != "xml" && kind != "pdf" {
		return nil, domain.ErrInvalidKey
	}
	belongs, err := s.repo.BelongsToCompany(ctx, companyID, invoiceID)
	if err != nil {
		return nil, err
	}
	if !belongs {
		return nil, domain.ErrNotFound
	}
	file, err := s.repo.FindFile(ctx, companyID, invoiceID, kind)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, domain.ErrNotFound
	}
	return file, err
}
