package batch

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"

	"github.com/brandsrx/supay/internal/ports"
)

// Packer is shared by individual, offline, package and bulk emission.
type Packer struct{}

var _ ports.LotPacker = Packer{}

func (Packer) PackDocument(xml []byte) (ports.PackedLot, error) {
	if len(xml) == 0 {
		return ports.PackedLot{}, fmt.Errorf("XML vacío")
	}
	return compress(xml)
}

func (Packer) PackLot(documents [][]byte) (ports.PackedLot, error) {
	if len(documents) == 0 {
		return ports.PackedLot{}, fmt.Errorf("lote vacío")
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for i, doc := range documents {
		if len(doc) == 0 {
			return ports.PackedLot{}, fmt.Errorf("factura %d: XML vacío", i+1)
		}
		if err := tw.WriteHeader(&tar.Header{Name: fmt.Sprintf("factura_%d.xml", i+1), Mode: 0600, Size: int64(len(doc)), Typeflag: tar.TypeReg}); err != nil {
			return ports.PackedLot{}, err
		}
		if _, err := tw.Write(doc); err != nil {
			return ports.PackedLot{}, err
		}
	}
	if err := tw.Close(); err != nil {
		return ports.PackedLot{}, err
	}
	return compress(buf.Bytes())
}

func compress(data []byte) (ports.PackedLot, error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(data); err != nil {
		return ports.PackedLot{}, err
	}
	if err := gz.Close(); err != nil {
		return ports.PackedLot{}, err
	}
	hash := sha256.Sum256(buf.Bytes())
	return ports.PackedLot{Archive: base64.StdEncoding.EncodeToString(buf.Bytes()), Hash: hex.EncodeToString(hash[:])}, nil
}
