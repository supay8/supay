package batch_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"testing"

	"github.com/brandsrx/supay/internal/adapters/siat/batch"
)

func TestPackLotPreservesSignedBytesOrderAndCompressedDigest(t *testing.T) {
	docs := [][]byte{[]byte("<factura id=\"1\">á&amp;β</factura>"), []byte("<factura id=\"2\"><Signature>exact bytes</Signature></factura>")}
	p := batch.Packer{}
	packed, err := p.PackLot(docs)
	if err != nil {
		t.Fatal(err)
	}
	again, err := p.PackLot(docs)
	if err != nil || again != packed {
		t.Fatal("packing must be deterministic", err)
	}
	data, err := base64.StdEncoding.DecodeString(packed.Archive)
	if err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	if packed.Hash != hex.EncodeToString(sum[:]) {
		t.Fatal("hash must cover the compressed archive")
	}
	gz, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for i, want := range docs {
		header, err := tr.Next()
		if err != nil {
			t.Fatal(err)
		}
		got, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) || header.Typeflag != tar.TypeReg {
			t.Fatalf("document %d changed: %q", i, got)
		}
	}
	if _, err := tr.Next(); err != io.EOF {
		t.Fatalf("unexpected trailing entry: %v", err)
	}
}

func TestPackRejectsEmptyDocuments(t *testing.T) {
	p := batch.Packer{}
	if _, err := p.PackDocument(nil); err == nil {
		t.Fatal("empty XML accepted")
	}
	if _, err := p.PackLot(nil); err == nil {
		t.Fatal("empty lot accepted")
	}
	if _, err := p.PackLot([][]byte{[]byte("<xml/>"), nil}); err == nil {
		t.Fatal("empty lot member accepted")
	}
}
