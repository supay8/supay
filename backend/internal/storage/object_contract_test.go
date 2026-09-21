package storage_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/storage"
)

func storageContract(t *testing.T, s domain.Storage) {
	t.Helper()
	ctx := context.Background()
	key := fmt.Sprintf("companies/company1/invoices/invoice1/%d.xml", time.Now().UnixNano())
	if _, err := s.Stat(ctx, key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing stat: %v", err)
	}
	if _, _, err := s.Get(ctx, key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("missing get: %v", err)
	}
	for _, bad := range []string{"../escape", "/abs", "a/../b", "a\\b", "a//b", "a\x00b"} {
		if _, err := s.Put(ctx, bad, strings.NewReader("x"), domain.PutOptions{}); !errors.Is(err, domain.ErrInvalidKey) {
			t.Errorf("invalid key %q: %v", bad, err)
		}
	}
	const size = 8 << 20
	payload := bytes.Repeat([]byte("streaming-object"), size/len("streaming-object"))
	info, err := s.Put(ctx, key, bytes.NewReader(payload), domain.PutOptions{ContentType: "application/xml", Size: int64(len(payload))})
	if err != nil {
		t.Fatal(err)
	}
	want := sha256.Sum256(payload)
	if info.SHA256 != hex.EncodeToString(want[:]) || info.Size != int64(len(payload)) {
		t.Fatalf("put info: %+v", info)
	}
	if _, err := s.Put(ctx, key, strings.NewReader("overwrite"), domain.PutOptions{}); !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("overwrite: %v", err)
	}
	stat, err := s.Stat(ctx, key)
	if err != nil || stat.Size != info.Size || stat.SHA256 != info.SHA256 || stat.ContentType != info.ContentType {
		t.Fatalf("stat: %+v %v", stat, err)
	}
	body, got, err := s.Get(ctx, key)
	if err != nil {
		t.Fatal(err)
	}
	read, err := io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(read, payload) || got.Size != info.Size {
		t.Fatalf("get: %v", err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, key); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Stat(ctx, key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("deleted stat: %v", err)
	}
}

func TestLocalObjectStorageContract(t *testing.T) {
	s, err := storage.NewLocalObjectStorage(t.TempDir(), strings.Repeat("s", 32), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	storageContract(t, s)
}

func TestMemoryObjectStorageContract(t *testing.T) {
	storageContract(t, storage.NewMemoryObjectStorage())
}

func TestLocalObjectStorageTraversalAndAtomicity(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	s, err := storage.NewLocalObjectStorage(dir, strings.Repeat("s", 32), "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if err := os.Symlink(outside, dir+"/escape"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Put(context.Background(), "escape/file", strings.NewReader("x"), domain.PutOptions{}); err == nil {
		t.Fatal("symlink escaped root")
	}
	key := "companies/c/invoices/i/CUF.xml"
	if _, err := s.Put(context.Background(), key, &brokenReader{}, domain.PutOptions{}); err == nil {
		t.Fatal("expected interrupted upload")
	}
	if _, err := s.Stat(context.Background(), key); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("partial object: %v", err)
	}
}

type brokenReader struct{ read bool }

func (r *brokenReader) Read(p []byte) (int, error) {
	if r.read {
		return 0, io.ErrUnexpectedEOF
	}
	r.read = true
	copy(p, "partial")
	return 7, nil
}

func TestLocalPresign(t *testing.T) {
	s, err := storage.NewLocalObjectStorage(t.TempDir(), strings.Repeat("s", 32), "https://api.example")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	key := "companies/c/invoices/i/CUF.xml"
	_, err = s.Put(context.Background(), key, strings.NewReader("xml"), domain.PutOptions{})
	if err != nil {
		t.Fatal(err)
	}
	link, err := s.PresignGet(context.Background(), key, time.Minute, "invoice.xml")
	if err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(link)
	if err != nil {
		t.Fatal(err)
	}
	q := u.Query()
	if err := s.VerifySignedURL(q.Get("key"), q.Get("filename"), q.Get("expires"), q.Get("signature"), time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := s.VerifySignedURL(q.Get("key"), "tampered.xml", q.Get("expires"), q.Get("signature"), time.Now()); err == nil {
		t.Fatal("tampered URL accepted")
	}
	if err := s.VerifySignedURL(q.Get("key"), q.Get("filename"), q.Get("expires"), q.Get("signature"), time.Now().Add(2*time.Minute)); err == nil {
		t.Fatal("expired URL accepted")
	}
}
