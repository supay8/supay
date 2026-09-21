package storage

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime"
	"os"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	"github.com/brandsrx/supay/internal/domain"
)

type ObjectR2Config struct {
	AccountID, AccessKeyID, SecretAccessKey, Bucket, Endpoint string
}

type R2ObjectStorage struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

const r2OperationTimeout = 2 * time.Minute

func NewR2ObjectStorage(ctx context.Context, cfg ObjectR2Config) (*R2ObjectStorage, error) {
	if cfg.Bucket == "" || cfg.AccessKeyID == "" || cfg.SecretAccessKey == "" || (cfg.AccountID == "" && cfg.Endpoint == "") {
		return nil, fmt.Errorf("storage r2: R2_BUCKET, R2_ACCESS_KEY_ID, R2_SECRET_ACCESS_KEY y R2_ACCOUNT_ID (o R2_ENDPOINT) son obligatorios")
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "https://" + cfg.AccountID + ".r2.cloudflarestorage.com"
	}
	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion("auto"),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("storage r2: config: %w", err)
	}
	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		o.UsePathStyle = true
		// R2 does not support every optional checksum header the recent SDKs send
		// by default. Cloudflare's S3 compatibility table marks the SDK checksum
		// algorithm header unsupported for several operations; keep checksums on
		// required operations only. See https://developers.cloudflare.com/r2/api/s3/api/.
		o.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
		o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
	})
	return &R2ObjectStorage{client: client, presigner: s3.NewPresignClient(client), bucket: cfg.Bucket}, nil
}

func r2Error(err error) error {
	var api smithy.APIError
	if errors.As(err, &api) {
		switch api.ErrorCode() {
		case "NoSuchKey", "NotFound", "NoSuchBucket", "404":
			return domain.ErrNotFound
		case "PreconditionFailed", "ConditionalRequestConflict", "412":
			return domain.ErrAlreadyExists
		}
	}
	return err
}

func (s *R2ObjectStorage) Put(ctx context.Context, key string, reader io.Reader, opts domain.PutOptions) (domain.ObjectInfo, error) {
	if err := domain.ValidateObjectKey(key); err != nil {
		return domain.ObjectInfo{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, r2OperationTimeout)
	defer cancel()
	// Spool to disk: the SDK needs a seekable body for signed retries, while a
	// caller can supply an unbounded non-seekable stream. Memory stays bounded.
	f, err := os.CreateTemp("", "supay-object-*")
	if err != nil {
		return domain.ObjectInfo{}, err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	h := sha256.New()
	n, err := io.Copy(f, io.TeeReader(&contextReader{ctx, reader}, h))
	if err != nil {
		return domain.ObjectInfo{}, err
	}
	if opts.Size > 0 && opts.Size != n {
		return domain.ObjectInfo{}, fmt.Errorf("storage r2: size mismatch: expected %d, got %d", opts.Size, n)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return domain.ObjectInfo{}, err
	}
	digest := hex.EncodeToString(h.Sum(nil))
	metadata := make(map[string]string, len(opts.Metadata)+1)
	for k, v := range opts.Metadata {
		metadata[k] = v
	}
	metadata["sha256"] = digest
	input := &s3.PutObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), Body: f,
		ContentLength: aws.Int64(n), ContentType: aws.String(opts.ContentType), Metadata: metadata,
		IfNoneMatch: aws.String("*"),
	}
	_, err = s.client.PutObject(ctx, input)
	if err != nil {
		return domain.ObjectInfo{}, fmt.Errorf("storage r2: put: %w", r2Error(err))
	}
	return domain.ObjectInfo{Key: key, Size: n, ContentType: opts.ContentType, SHA256: digest, CreatedAt: time.Now().UTC()}, nil
}

func (s *R2ObjectStorage) Stat(ctx context.Context, key string) (domain.ObjectInfo, error) {
	if err := domain.ValidateObjectKey(key); err != nil {
		return domain.ObjectInfo{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, r2OperationTimeout)
	defer cancel()
	out, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		return domain.ObjectInfo{}, fmt.Errorf("storage r2: stat: %w", r2Error(err))
	}
	ct := aws.ToString(out.ContentType)
	if ct == "" {
		ct = mime.TypeByExtension(path.Ext(key))
	}
	return domain.ObjectInfo{Key: key, Size: aws.ToInt64(out.ContentLength), ContentType: ct, SHA256: out.Metadata["sha256"], CreatedAt: aws.ToTime(out.LastModified)}, nil
}

func (s *R2ObjectStorage) Get(ctx context.Context, key string) (io.ReadCloser, domain.ObjectInfo, error) {
	if err := domain.ValidateObjectKey(key); err != nil {
		return nil, domain.ObjectInfo{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, r2OperationTimeout)
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	if err != nil {
		cancel()
		return nil, domain.ObjectInfo{}, fmt.Errorf("storage r2: get: %w", r2Error(err))
	}
	info := domain.ObjectInfo{Key: key, Size: aws.ToInt64(out.ContentLength), ContentType: aws.ToString(out.ContentType), SHA256: out.Metadata["sha256"], CreatedAt: aws.ToTime(out.LastModified)}
	return &cancelReadCloser{ReadCloser: out.Body, cancel: cancel}, info, nil
}

type cancelReadCloser struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (r *cancelReadCloser) Close() error { r.cancel(); return r.ReadCloser.Close() }

func (s *R2ObjectStorage) Delete(ctx context.Context, key string) error {
	if err := domain.ValidateObjectKey(key); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, r2OperationTimeout)
	defer cancel()
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return r2Error(err)
}

func (s *R2ObjectStorage) PresignGet(ctx context.Context, key string, ttl time.Duration, filename string) (string, error) {
	if err := domain.ValidateObjectKey(key); err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, r2OperationTimeout)
	defer cancel()
	if ttl <= 0 || ttl > 24*time.Hour {
		return "", fmt.Errorf("storage r2: invalid presign ttl")
	}
	if _, err := s.Stat(ctx, key); err != nil {
		return "", err
	}
	filename = strings.ReplaceAll(strings.ReplaceAll(filename, "\r", ""), "\n", "")
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": filename})
	if disposition == "" {
		return "", domain.ErrInvalidKey
	}
	request, err := s.presigner.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key), ResponseContentDisposition: aws.String(disposition)}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", fmt.Errorf("storage r2: presign: %w", err)
	}
	return request.URL, nil
}

var _ domain.Storage = (*R2ObjectStorage)(nil)
