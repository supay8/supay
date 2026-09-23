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
	file, _, err := s.SaveWithStatus(ctx, companyID, invoiceID, cuf, kind, reader, size)
	return file, err
}

// SaveWithStatus stores the immutable object before its metadata row and reports
// whether this call created it. Callers that stage files before a larger
// database transaction can compensate only objects created by that attempt.
func (s *InvoiceFileService) SaveWithStatus(ctx context.Context, companyID, invoiceID, cuf, kind string, reader io.Reader, size int64) (*domain.InvoiceFile, bool, error) {
	key, err := domain.InvoiceObjectKey(companyID, invoiceID, cuf, kind)
	if err != nil {
		return nil, false, err
	}
	belongs, err := s.repo.BelongsToCompany(ctx, companyID, invoiceID)
	if err != nil {
		return nil, false, err
	}
	if !belongs {
		return nil, false, domain.ErrNotFound
	}
	contentType := "application/xml"
	if kind == "pdf" {
		contentType = "application/pdf"
	}
	hash := sha256.New()
	info, err := s.storage.Put(ctx, key, io.TeeReader(reader, hash), domain.PutOptions{ContentType: contentType, Size: size})
	if errors.Is(err, domain.ErrAlreadyExists) {
		// Algunos backends detectan la colisión antes de consumir el reader
		// (memoria) y otros después de consumirlo (local/R2). Completar desde la
		// posición actual produce el hash correcto en ambos casos.
		if _, hashErr := io.Copy(hash, reader); hashErr != nil {
			return nil, false, hashErr
		}
		digest := hex.EncodeToString(hash.Sum(nil))
		stored, statErr := s.storage.Stat(ctx, key)
		if statErr != nil {
			return nil, false, fmt.Errorf("verify existing invoice object %s: %w", key, statErr)
		}
		if stored.SHA256 != digest || (size > 0 && stored.Size != size) {
			return nil, false, fmt.Errorf("invoice object exists with different content; reconcile %s: %w", key, domain.ErrAlreadyExists)
		}
		existing, findErr := s.repo.FindFile(ctx, companyID, invoiceID, kind)
		if findErr == nil && existing.StorageKey == key && existing.SHA256 == digest {
			return existing, false, nil
		}
		if errors.Is(findErr, domain.ErrNotFound) {
			file := &domain.InvoiceFile{CompanyID: companyID, InvoiceID: invoiceID, Kind: kind, StorageKey: key, SHA256: digest, Size: stored.Size, ContentType: contentType, CreatedAt: stored.CreatedAt}
			if createErr := s.repo.CreateFile(ctx, file); createErr == nil {
				// El objeto ya existía; no debe eliminarse si una operación superior
				// necesita compensar su propio trabajo.
				return file, false, nil
			}
		}
		return nil, false, fmt.Errorf("invoice object exists without matching metadata; reconcile %s: %w", key, domain.ErrAlreadyExists)
	}
	if err != nil {
		return nil, false, err
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
		return nil, false, fmt.Errorf("invoice file metadata: %w", err)
	}
	return file, true, nil
}

// RemoveCreated compensates a staged object that could not be committed by its
// enclosing operation. The storage key match prevents deleting a newer file.
func (s *InvoiceFileService) RemoveCreated(ctx context.Context, file *domain.InvoiceFile) error {
	if file == nil {
		return nil
	}
	if err := s.repo.DeleteFile(ctx, file.CompanyID, file.InvoiceID, file.Kind, file.StorageKey); err != nil {
		return err
	}
	return s.storage.Delete(ctx, file.StorageKey)
}

func (s *InvoiceFileService) ReadAll(ctx context.Context, companyID, invoiceID, kind string) ([]byte, *domain.InvoiceFile, error) {
	file, err := s.file(ctx, companyID, invoiceID, kind)
	if err != nil {
		return nil, nil, err
	}
	body, _, err := s.storage.Get(ctx, file.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		return nil, nil, err
	}
	return data, file, nil
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
