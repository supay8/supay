package sandbox

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"

	"github.com/brandsrx/supay/internal/ports"
)

var _ ports.FiscalBatchPreparer = (*FiscalService)(nil)

type preparedBatch struct {
	service *FiscalService
	bulk    bool
	result  ports.FiscalPackageResult
}

func (s *FiscalService) PrepareBulk(ctx context.Context, bulk ports.FiscalBulk) (ports.FiscalBulk, error) {
	docs, prepared, err := s.prepareBatch(ctx, bulk.Facturas, true)
	if err != nil {
		return ports.FiscalBulk{}, err
	}
	bulk.Facturas, bulk.Prepared = docs, prepared
	bulk.Archivo, bulk.HashArchivo = prepared.result.Archivo, prepared.result.HashArchivo
	return bulk, nil
}

func (s *FiscalService) PreparePackage(ctx context.Context, pkg ports.FiscalPackage) (ports.FiscalPackage, error) {
	docs, prepared, err := s.prepareBatch(ctx, pkg.Facturas, false)
	if err != nil {
		return ports.FiscalPackage{}, err
	}
	pkg.Facturas, pkg.Prepared = docs, prepared
	pkg.Archivo, pkg.HashArchivo = prepared.result.Archivo, prepared.result.HashArchivo
	return pkg, nil
}

func (s *FiscalService) prepareBatch(ctx context.Context, documents []ports.FiscalDocument, bulk bool) ([]ports.FiscalDocument, *preparedBatch, error) {
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	limit := 500
	if bulk {
		limit = 1000
	}
	if len(documents) == 0 || len(documents) > limit {
		return nil, nil, fmt.Errorf("sandbox: el lote debe contener entre 1 y %d facturas", limit)
	}
	docs := append([]ports.FiscalDocument(nil), documents...)
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	cufs := make([]string, len(docs))
	for i := range docs {
		d := &docs[i]
		if bulk && (d.Cuf != "" || d.XML != "") {
			return nil, nil, fmt.Errorf("sandbox: no se puede regenerar un documento fiscal ya emitido")
		}
		if d.XML == "" {
			d.Cuf = s.nextCode("FAKE-CUF-BATCH")
			d.XML = fmt.Sprintf("<fake-batch xmlns=\"sandbox\"><cuf>%s</cuf><numeroFactura>%d</numeroFactura></fake-batch>", d.Cuf, d.NumeroFactura)
		}
		if d.Cuf == "" {
			return nil, nil, fmt.Errorf("sandbox: el XML persistido requiere su CUF")
		}
		var err error
		d.Archivo, d.HashArchivo, err = compress([]byte(d.XML))
		if err != nil {
			return nil, nil, err
		}
		cufs[i] = d.Cuf
		if err := writer.WriteHeader(&tar.Header{Name: fmt.Sprintf("factura_%d.xml", i+1), Mode: 0600, Size: int64(len(d.XML))}); err != nil {
			return nil, nil, err
		}
		if _, err := writer.Write([]byte(d.XML)); err != nil {
			return nil, nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, nil, err
	}
	archive, hash, err := compress(buffer.Bytes())
	if err != nil {
		return nil, nil, err
	}
	return docs, &preparedBatch{service: s, bulk: bulk, result: ports.FiscalPackageResult{
		Archivo: archive, HashArchivo: hash, CantidadFacturas: len(docs), Cufs: cufs,
	}}, nil
}

func compress(data []byte) (archive, hash string, err error) {
	var compressed bytes.Buffer
	writer := gzip.NewWriter(&compressed)
	if _, err := writer.Write(data); err != nil {
		return "", "", err
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}
	digest := sha256.Sum256(compressed.Bytes())
	return base64.StdEncoding.EncodeToString(compressed.Bytes()), fmt.Sprintf("%x", digest), nil
}

func (s *FiscalService) sendPrepared(ctx context.Context, opaque any, bulk bool) (ports.FiscalPackageResult, error) {
	if err := ctx.Err(); err != nil {
		return ports.FiscalPackageResult{}, err
	}
	prepared, ok := opaque.(*preparedBatch)
	if !ok || prepared == nil || prepared.service != s || prepared.bulk != bulk {
		return ports.FiscalPackageResult{}, fmt.Errorf("sandbox: preparación incompatible con el envío")
	}
	result := s.packageResult(prepared.result.CantidadFacturas)
	result.Archivo, result.HashArchivo, result.Cufs = prepared.result.Archivo, prepared.result.HashArchivo, prepared.result.Cufs
	return result, nil
}
