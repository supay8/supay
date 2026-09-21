package storage

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
)

type LocalObjectStorage struct {
	root    *os.Root
	secret  []byte
	baseURL string
}

func NewLocalObjectStorage(dir, signingSecret, baseURL string) (*LocalObjectStorage, error) {
	exePath, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("storage local: os.Executable: %w", err)
	}
	if dir == "" {
		return nil, fmt.Errorf("STORAGE_LOCAL_PATH es obligatorio")
	}
	storagePath := filepath.Join(exePath, "..", dir)
	if len(signingSecret) < 32 {
		return nil, fmt.Errorf("STORAGE_SIGNING_SECRET debe tener al menos 32 caracteres")
	}
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		return nil, fmt.Errorf("storage local: mkdir: %w", err)
	}
	root, err := os.OpenRoot(storagePath)
	if err != nil {
		return nil, fmt.Errorf("storage local: open root: %w", err)
	}
	return &LocalObjectStorage{root: root, secret: []byte(signingSecret), baseURL: strings.TrimRight(baseURL, "/")}, nil
}

func (s *LocalObjectStorage) Close() error { return s.root.Close() }

func (s *LocalObjectStorage) Put(ctx context.Context, key string, reader io.Reader, opts domain.PutOptions) (domain.ObjectInfo, error) {
	if err := domain.ValidateObjectKey(key); err != nil {
		return domain.ObjectInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.ObjectInfo{}, err
	}
	if err := s.root.MkdirAll(path.Dir(key), 0700); err != nil {
		return domain.ObjectInfo{}, err
	}
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		return domain.ObjectInfo{}, err
	}
	tmp := path.Join(path.Dir(key), ".upload-"+hex.EncodeToString(nonce[:]))
	f, err := s.root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return domain.ObjectInfo{}, err
	}
	defer s.root.Remove(tmp)
	h := sha256.New()
	n, copyErr := io.Copy(f, io.TeeReader(&contextReader{ctx, reader}, h))
	closeErr := f.Close()
	if copyErr != nil {
		return domain.ObjectInfo{}, copyErr
	}
	if closeErr != nil {
		return domain.ObjectInfo{}, closeErr
	}
	if opts.Size > 0 && opts.Size != n {
		return domain.ObjectInfo{}, fmt.Errorf("storage local: size mismatch: expected %d, got %d", opts.Size, n)
	}
	ct := opts.ContentType
	if ct == "" {
		ct = mime.TypeByExtension(path.Ext(key))
	}
	if ct == "" {
		ct = "application/octet-stream"
	}
	fileInfo, err := s.root.Stat(tmp)
	if err != nil {
		return domain.ObjectInfo{}, err
	}
	info := domain.ObjectInfo{Key: key, Size: n, ContentType: ct, SHA256: hex.EncodeToString(h.Sum(nil)), CreatedAt: fileInfo.ModTime()}
	// Link is an atomic create-if-absent; rename would overwrite an immutable XML.
	if err := s.root.Link(tmp, key); err != nil {
		if errors.Is(err, os.ErrExist) {
			return domain.ObjectInfo{}, domain.ErrAlreadyExists
		}
		return domain.ObjectInfo{}, fmt.Errorf("storage local: link: %w", err)
	}
	if err := s.writeMeta(tmp+".meta", key+".meta", info); err != nil {
		_ = s.root.Remove(key)
		return domain.ObjectInfo{}, err
	}
	return info, nil
}

func (s *LocalObjectStorage) writeMeta(tmp, target string, info domain.ObjectInfo) error {
	f, err := s.root.OpenFile(tmp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	defer s.root.Remove(tmp)
	encodeErr := json.NewEncoder(f).Encode(info)
	closeErr := f.Close()
	if encodeErr != nil {
		return encodeErr
	}
	if closeErr != nil {
		return closeErr
	}
	return s.root.Link(tmp, target)
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (s *LocalObjectStorage) Stat(ctx context.Context, key string) (domain.ObjectInfo, error) {
	if err := domain.ValidateObjectKey(key); err != nil {
		return domain.ObjectInfo{}, err
	}
	if err := ctx.Err(); err != nil {
		return domain.ObjectInfo{}, err
	}
	info, err := s.root.Stat(key)
	if errors.Is(err, os.ErrNotExist) {
		return domain.ObjectInfo{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.ObjectInfo{}, err
	}
	if !info.Mode().IsRegular() {
		return domain.ObjectInfo{}, domain.ErrInvalidKey
	}
	ct := mime.TypeByExtension(path.Ext(key))
	if ct == "" {
		ct = "application/octet-stream"
	}
	result := domain.ObjectInfo{Key: key, Size: info.Size(), ContentType: ct, CreatedAt: info.ModTime()}
	if metadata, err := s.root.Open(key + ".meta"); err == nil {
		var stored domain.ObjectInfo
		decodeErr := json.NewDecoder(io.LimitReader(metadata, 4096)).Decode(&stored)
		_ = metadata.Close()
		if decodeErr == nil && stored.Key == key && stored.Size == result.Size {
			result.ContentType, result.SHA256, result.CreatedAt = stored.ContentType, stored.SHA256, stored.CreatedAt
		}
	}
	return result, nil
}

func (s *LocalObjectStorage) Get(ctx context.Context, key string) (io.ReadCloser, domain.ObjectInfo, error) {
	info, err := s.Stat(ctx, key)
	if err != nil {
		return nil, domain.ObjectInfo{}, err
	}
	f, err := s.root.Open(key)
	if errors.Is(err, os.ErrNotExist) {
		return nil, domain.ObjectInfo{}, domain.ErrNotFound
	}
	if err != nil {
		return nil, domain.ObjectInfo{}, err
	}
	return &contextReadCloser{ctx: ctx, ReadCloser: f}, info, nil
}

type contextReadCloser struct {
	ctx context.Context
	io.ReadCloser
}

func (r *contextReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.ReadCloser.Read(p)
}

func (s *LocalObjectStorage) Delete(ctx context.Context, key string) error {
	if err := domain.ValidateObjectKey(key); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.root.Remove(key); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := s.root.Remove(key + ".meta"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *LocalObjectStorage) signature(key string, expiry int64, filename string) string {
	h := hmac.New(sha256.New, s.secret)
	fmt.Fprintf(h, "%s\n%d\n%s", key, expiry, filename)
	return hex.EncodeToString(h.Sum(nil))
}

func (s *LocalObjectStorage) PresignGet(ctx context.Context, key string, ttl time.Duration, filename string) (string, error) {
	if _, err := s.Stat(ctx, key); err != nil {
		return "", err
	}
	if ttl <= 0 || ttl > 24*time.Hour {
		return "", fmt.Errorf("storage local: invalid presign ttl")
	}
	filename = strings.ReplaceAll(strings.ReplaceAll(filename, "\r", ""), "\n", "")
	expiry := time.Now().Add(ttl).Unix()
	q := url.Values{"key": {key}, "expires": {strconv.FormatInt(expiry, 10)}, "filename": {filename}}
	q.Set("signature", s.signature(key, expiry, filename))
	return s.baseURL + "/storage/download?" + q.Encode(), nil
}

func (s *LocalObjectStorage) VerifySignedURL(key, filename, expiryText, signature string, now time.Time) error {
	if err := domain.ValidateObjectKey(key); err != nil {
		return err
	}
	expiry, err := strconv.ParseInt(expiryText, 10, 64)
	if err != nil || expiry < now.Unix() {
		return domain.ErrNotFound
	}
	provided, err := hex.DecodeString(signature)
	if err != nil || !hmac.Equal(provided, mustDecodeHex(s.signature(key, expiry, filename))) {
		return domain.ErrNotFound
	}
	return nil
}

func mustDecodeHex(s string) []byte { b, _ := hex.DecodeString(s); return b }

var _ domain.Storage = (*LocalObjectStorage)(nil)
