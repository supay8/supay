package domain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strings"
	"time"
)

var (
	ErrNotFound      = errors.New("storage: object not found")
	ErrAlreadyExists = errors.New("storage: object already exists")
	ErrInvalidKey    = errors.New("storage: invalid key")
)

type PutOptions struct {
	ContentType string
	Size        int64 // 0 when unknown
	Metadata    map[string]string
}

type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
	SHA256      string
	CreatedAt   time.Time
}

type Storage interface {
	Put(context.Context, string, io.Reader, PutOptions) (ObjectInfo, error)
	Get(context.Context, string) (io.ReadCloser, ObjectInfo, error)
	Stat(context.Context, string) (ObjectInfo, error)
	Delete(context.Context, string) error
	PresignGet(context.Context, string, time.Duration, string) (string, error)
}

// ValidateObjectKey accepts only portable relative keys and rejects traversal
// before the key reaches either a filesystem or S3 adapter.
func ValidateObjectKey(key string) error {
	if key == "" || strings.HasPrefix(key, "/") || strings.ContainsAny(key, "\\\x00") || path.Clean(key) != key {
		return ErrInvalidKey
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." || strings.Contains(part, ":") {
			return ErrInvalidKey
		}
	}
	return nil
}

var objectID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
var cufID = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// InvoiceObjectKey is the sole constructor for fiscal document object keys.
func InvoiceObjectKey(companyID, invoiceID, cuf, kind string) (string, error) {
	if !objectID.MatchString(companyID) || !objectID.MatchString(invoiceID) || !cufID.MatchString(cuf) || (kind != "xml" && kind != "pdf") {
		return "", ErrInvalidKey
	}
	key := fmt.Sprintf("companies/%s/invoices/%s/%s.%s", companyID, invoiceID, cuf, kind)
	return key, ValidateObjectKey(key)
}
