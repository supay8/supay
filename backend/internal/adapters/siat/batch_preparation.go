package siat

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"github.com/brandsrx/supay/internal/ports"
)

var _ ports.FiscalBatchPreparer = (*FiscalAdapter)(nil)

func validarIdentidadLote(facturas []SolicitudFactura, lote SolicitudPaqueteFactura, historico bool) error {
	perfil, err := PerfilSectorLayout(lote.sector(), lote.Layout)
	if err != nil {
		return err
	}
	numeros := make(map[int64]struct{}, len(facturas))
	for i, f := range facturas {
		sector := f.CodigoDocumentoSector
		if sector <= 0 {
			sector = SectorCompraVenta
		}
		if f.Nit != lote.Nit || f.CodigoAmbiente != lote.CodigoAmbiente || f.CodigoSistema != lote.CodigoSistema ||
			f.Modalidad != lote.Modalidad || f.CodigoSucursal != lote.CodigoSucursal || f.CodigoPuntoVenta != lote.CodigoPuntoVenta ||
			sector != perfil.Codigo || strings.TrimSpace(f.Layout) != strings.TrimSpace(lote.Layout) ||
			perfil.TipoDocumentoResuelto(f.CodigoTipoFactura) != perfil.TipoDocumentoResuelto(lote.CodigoTipoFactura) {
			return fmt.Errorf("factura %d: la identidad fiscal no coincide con el lote", i+1)
		}
		if !historico && (f.Cufd != lote.Cufd || f.Cuis != lote.Cuis || f.CodigoControl != lote.CodigoControl) {
			return fmt.Errorf("factura %d: las credenciales fiscales no coinciden con el lote", i+1)
		}
		if _, exists := numeros[f.NumeroFactura]; exists {
			return fmt.Errorf("factura %d: número de factura duplicado en el lote", i+1)
		}
		numeros[f.NumeroFactura] = struct{}{}
	}
	return nil
}

func (a *FiscalAdapter) PrepareBulk(ctx context.Context, bulk ports.FiscalBulk) (ports.FiscalBulk, error) {
	prepared, err := a.svc.prepararMasiva(ctx, toSiatMasiva(bulk))
	if err != nil {
		return ports.FiscalBulk{}, err
	}
	bulk.Prepared = prepared
	bulk.Archivo, bulk.HashArchivo = prepared.result.Archivo, prepared.result.HashArchivo
	bulk.Facturas = documentosPreparados(bulk.Facturas, prepared.req.Facturas)
	return bulk, nil
}

func (a *FiscalAdapter) PreparePackage(ctx context.Context, pkg ports.FiscalPackage) (ports.FiscalPackage, error) {
	prepared, err := a.svc.prepararPaquete(ctx, toSiatPaquete(pkg))
	if err != nil {
		return ports.FiscalPackage{}, err
	}
	pkg.Prepared = prepared
	pkg.Archivo, pkg.HashArchivo = prepared.result.Archivo, prepared.result.HashArchivo
	pkg.Facturas = documentosPreparados(pkg.Facturas, prepared.req.Facturas)
	return pkg, nil
}

func documentosPreparados(original []ports.FiscalDocument, prepared []SolicitudFactura) []ports.FiscalDocument {
	docs := append([]ports.FiscalDocument(nil), original...)
	for i, p := range prepared {
		d := &docs[i]
		d.CodigoAmbiente, d.CodigoSistema, d.Nit = p.CodigoAmbiente, p.CodigoSistema, p.Nit
		d.Modalidad, d.CodigoSucursal, d.CodigoPuntoVenta = p.Modalidad, p.CodigoSucursal, p.CodigoPuntoVenta
		d.Cuis, d.Cufd, d.CodigoControl = p.Cuis, p.Cufd, p.CodigoControl
		d.CodigoDocumentoSector, d.Layout, d.CodigoTipoFactura = p.CodigoDocumentoSector, p.Layout, p.CodigoTipoFactura
		d.XML, d.Cuf, d.Archivo, d.HashArchivo = p.XML, p.Cuf, p.Archivo, p.HashArchivo
	}
	return docs
}

// recuperarDocumentosLote extrae los bytes ya serializados y firmados por
// WithFacturas del SDK. Así la auditoría guarda exactamente lo que se enviará,
// sin repetir la firma ni mantener una segunda implementación de sus builders.
func recuperarDocumentosLote(archivo string, facturas []SolicitudFactura, cufs []string) error {
	if len(cufs) != len(facturas) {
		return fmt.Errorf("cantidad de CUF inconsistente con las facturas preparadas")
	}
	compressed, err := base64.StdEncoding.DecodeString(archivo)
	if err != nil {
		return fmt.Errorf("archivo de lote inválido: %w", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		return fmt.Errorf("gzip de lote inválido: %w", err)
	}
	defer gz.Close()
	reader := tar.NewReader(gz)
	for i := range facturas {
		header, err := reader.Next()
		if err != nil {
			return fmt.Errorf("leer factura %d del lote: %w", i+1, err)
		}
		if header.Name != fmt.Sprintf("factura_%d.xml", i+1) || header.Typeflag != tar.TypeReg {
			return fmt.Errorf("entrada inesperada en lote preparado: %s", header.Name)
		}
		data, err := io.ReadAll(reader)
		if err != nil {
			return fmt.Errorf("leer XML de factura %d: %w", i+1, err)
		}
		individual, hash, err := empaquetaArchivo(data)
		if err != nil {
			return fmt.Errorf("comprimir factura %d: %w", i+1, err)
		}
		facturas[i].XML, facturas[i].Cuf = string(data), cufs[i]
		facturas[i].Archivo, facturas[i].HashArchivo = individual, hash
	}
	if _, err := reader.Next(); err != io.EOF {
		return fmt.Errorf("el lote preparado contiene entradas adicionales o es inválido")
	}
	// Consume el tráiler gzip para verificar también su CRC.
	if _, err := io.Copy(io.Discard, gz); err != nil {
		return fmt.Errorf("gzip de lote inválido: %w", err)
	}
	return nil
}
