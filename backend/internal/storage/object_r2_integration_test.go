package storage_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/storage"
)

func TestMinIOObjectStorageContract(t *testing.T) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("set MINIO_ENDPOINT and run docker compose --profile storage-test up -d minio")
	}
	// MinIO is S3-compatible for contract tests but is not identical to R2.
	s, err := storage.NewR2ObjectStorage(context.Background(), storage.ObjectR2Config{Endpoint: endpoint, AccessKeyID: os.Getenv("MINIO_ACCESS_KEY"), SecretAccessKey: os.Getenv("MINIO_SECRET_KEY"), Bucket: os.Getenv("MINIO_BUCKET")})
	if err != nil {
		t.Fatal(err)
	}
	storageContract(t, s)
}

func TestMinIOCertificateAdapter(t *testing.T) {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("MINIO_ENDPOINT required")
	}
	certs, err := storage.NewR2CertStorage(storage.R2Config{Endpoint: endpoint, AccessKeyID: os.Getenv("MINIO_ACCESS_KEY"), SecretAccessKey: os.Getenv("MINIO_SECRET_KEY"), Bucket: os.Getenv("MINIO_BUCKET")})
	if err != nil {
		t.Fatal(err)
	}
	key := fmt.Sprintf("certs/test/%d.p12.enc", time.Now().UnixNano())
	ref, err := certs.Put(context.Background(), key, []byte("encrypted-cert"))
	if err != nil {
		t.Fatal(err)
	}
	defer certs.Delete(context.Background(), ref)
	got, err := certs.Get(context.Background(), ref)
	if err != nil || string(got) != "encrypted-cert" {
		t.Fatalf("certificate read: %q %v", got, err)
	}
}

func TestRealR2ObjectStorageContract(t *testing.T) {
	if os.Getenv("R2_INTEGRATION_TEST") != "1" {
		t.Skip("R2_INTEGRATION_TEST=1 required")
	}
	s, err := storage.NewR2ObjectStorage(context.Background(), storage.ObjectR2Config{AccountID: os.Getenv("R2_ACCOUNT_ID"), AccessKeyID: os.Getenv("R2_ACCESS_KEY_ID"), SecretAccessKey: os.Getenv("R2_SECRET_ACCESS_KEY"), Bucket: os.Getenv("R2_BUCKET")})
	if err != nil {
		t.Fatal(err)
	}
	storageContract(t, s)
}
