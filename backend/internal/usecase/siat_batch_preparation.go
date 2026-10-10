package usecase

import (
	"context"
	"crypto/sha256"
	"fmt"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
	"strconv"
	"strings"
)

func (uc *SiatUsecase) prepareInvoiceBatches(ctx context.Context, company *domain.Company, pos *domain.PointOfSale, current *domain.Cufd, invoices []*domain.Invoice, event *domain.ContingencyEvent, kind domain.SentPackageType) ([]invoiceBatch, error) {
	if current == nil || current.ID == "" || current.PointOfSaleID != pos.ID || pos.Cuis == nil || *pos.Cuis == "" {
		return nil, domain.NewConflictError("No se pudieron resolver las credenciales del punto de venta")
	}
	limit, emission := fiscal.MaxFacturasMasiva, fiscal.EmisionMasiva
	if kind == domain.PackageTypePaquete {
		limit, emission = fiscal.MaxFacturasPorPaquete, fiscal.EmisionPaqueteOffline
	}
	type groupKey struct {
		sector, invoiceType, modality int
		layout, cufdID                string
	}
	groups := make(map[groupKey]int)
	batches := make([]invoiceBatch, 0)
	for _, inv := range invoices {
		credential := current
		var persistedXML string
		if kind == domain.PackageTypePaquete {
			credential = &inv.CufdRecord
			if inv.Cuf == nil || strings.TrimSpace(*inv.Cuf) == "" {
				return nil, domain.NewConflictError("La factura offline " + inv.ID + " no conserva su XML y CUF originales")
			}
			if uc.fileService == nil {
				return nil, domain.NewConflictError("El storage de documentos fiscales no está configurado")
			}
			data, file, err := uc.fileService.ReadAll(ctx, inv.CompanyId, inv.ID, "xml")
			if err != nil {
				return nil, domain.NewConflictError("La factura offline " + inv.ID + " no conserva su XML firmado en storage")
			}
			persistedXML = string(data)
			if strings.TrimSpace(persistedXML) == "" || (inv.XmlHash != nil && *inv.XmlHash != "" && *inv.XmlHash != file.SHA256) {
				return nil, domain.NewConflictError("El XML firmado de la factura offline " + inv.ID + " no coincide con su metadata")
			}
			if credential.ID == "" || credential.ID != inv.CufdId || credential.PointOfSaleID != pos.ID {
				return nil, domain.NewConflictError("No se pudo resolver el CUFD histórico de la factura " + inv.ID)
			}
		}
		if credential.Cufd == "" || credential.ControlCode == "" || inv.IssueDate.IsZero() ||
			(!credential.ValidFrom.IsZero() && inv.IssueDate.Before(credential.ValidFrom)) ||
			(!credential.ValidTo.IsZero() && inv.IssueDate.After(credential.ValidTo)) {
			return nil, domain.NewConflictError("La factura " + inv.ID + " no tiene un CUFD válido para su fecha de emisión")
		}
		profile, err := fiscal.PerfilSectorLayout(inv.CodigoDocumentoSector, inv.Layout)
		if err != nil {
			return nil, domain.NewBadRequestError(err.Error())
		}
		var doc ports.FiscalDocument
		if kind == domain.PackageTypePaquete {
			// El XML emitido es la fuente de verdad. Los cambios posteriores
			// del cliente o catálogo no deben alterar una factura offline.
			if inv.Modalidad <= 0 || inv.CodigoTipoFactura <= 0 {
				return nil, domain.NewConflictError("La factura offline " + inv.ID + " no conserva su modalidad o tipo fiscal")
			}
			doc = ports.FiscalDocument{
				CodigoAmbiente: company.Ambiente.CodigoAmbiente(), CodigoSistema: "",
				Nit: company.Nit, Modalidad: inv.Modalidad, NumeroFactura: int64(inv.InvoiceNumber),
				CodigoSucursal: pos.CodigoSucursal, CodigoDocumentoSector: inv.CodigoDocumentoSector,
				FechaEmision: inv.IssueDate, XML: persistedXML, Cuf: *inv.Cuf,
			}
		} else {
			doc, err = uc.solicitudDesdeInvoice(inv, company, pos, credential)
			if err != nil {
				return nil, err
			}
		}
		if doc.Modalidad == 0 {
			doc.Modalidad = uc.effectiveModalidadForCompany(company)
		}
		if doc.Modalidad != fiscal.ModalidadElectronica && doc.Modalidad != fiscal.ModalidadComputarizada {
			return nil, domain.NewConflictError("La factura " + inv.ID + " no tiene una modalidad válida")
		}
		if err := profile.ValidarModalidad(doc.Modalidad); err != nil {
			return nil, domain.NewConflictError(err.Error())
		}
		doc.Layout = profile.Layout
		doc.CodigoTipoFactura = profile.TipoDocumentoResuelto(inv.CodigoTipoFactura)
		doc.Cufd, doc.CodigoControl, doc.Cuis = credential.Cufd, credential.ControlCode, *pos.Cuis
		doc.CodigoPuntoVenta = resolveCodigoPuntoVenta(pos)
		if kind == domain.PackageTypePaquete {
			doc.XML, doc.Cuf = persistedXML, *inv.Cuf
		}
		key := groupKey{doc.CodigoDocumentoSector, doc.CodigoTipoFactura, doc.Modalidad, doc.Layout, credential.ID}
		idx, exists := groups[key]
		if !exists || len(batches[idx].documents) == limit {
			pkg := &domain.SentPackage{
				CompanyId: company.ID, PointOfSaleId: pos.ID, Type: kind,
				CodigoDocumentoSector: doc.CodigoDocumentoSector, CodigoTipoFactura: doc.CodigoTipoFactura,
				CodigoEmision: emission, Modalidad: doc.Modalidad, Layout: doc.Layout,
				Cufd: current.Cufd, CufdID: current.ID, Cuis: *pos.Cuis, Status: domain.PackageStatusSending,
			}
			if event != nil {
				code, _ := strconv.ParseInt(*event.SiatEventCode, 10, 64)
				pkg.CodigoEvento, pkg.ContingencyEventId = &code, &event.ID
			}
			idx = len(batches)
			groups[key] = idx
			batches = append(batches, invoiceBatch{pkg: pkg})
		}
		batches[idx].documents = append(batches[idx].documents, doc)
		batches[idx].pkg.InvoiceIDs = append(batches[idx].pkg.InvoiceIDs, inv.ID)
		batches[idx].pkg.CantidadFacturas++
	}
	return batches, nil
}

// Preparar todos los lotes antes de reservar evita que un error local en una
// factura tardía deje envíos parciales o facturas atascadas en SENDING.
func prepareBatchPayloads(ctx context.Context, svc ports.FiscalOperations, batches []invoiceBatch, company *domain.Company, pos *domain.PointOfSale, current *domain.Cufd) error {
	preparer, ok := svc.(ports.FiscalBatchPreparer)
	if !ok {
		return domain.NewConflictError("El servicio fiscal no permite preparar lotes antes de enviarlos")
	}
	for i := range batches {
		if err := ctx.Err(); err != nil {
			return err
		}
		batch := &batches[i]
		req := fiscalBatchRequest(batch.pkg, company, pos)
		req.CodigoControl, req.Facturas = current.ControlCode, batch.documents
		var documents []ports.FiscalDocument
		var err error
		if batch.pkg.Type == domain.PackageTypeMasiva {
			batch.bulk, err = preparer.PrepareBulk(ctx, req)
			documents = batch.bulk.Facturas
			batch.pkg.HashArchivo = batch.bulk.HashArchivo
		} else {
			batch.pack, err = preparer.PreparePackage(ctx, packageRequest(req, batch.pkg.CodigoEvento))
			documents = batch.pack.Facturas
			batch.pkg.HashArchivo = batch.pack.HashArchivo
		}
		if err != nil {
			return domain.NewBadRequestError("No se pudo preparar el lote: " + err.Error())
		}
		if len(documents) != len(batch.pkg.InvoiceIDs) || batch.pkg.HashArchivo == "" {
			return fmt.Errorf("el servicio fiscal preparó un lote incompleto")
		}
		batch.pkg.Documents = make([]domain.BatchInvoiceDocument, len(documents))
		for j, doc := range documents {
			if doc.XML == "" || doc.Cuf == "" || doc.Archivo == "" || doc.HashArchivo == "" {
				return fmt.Errorf("el servicio fiscal no preparó los documentos de la factura %s", batch.pkg.InvoiceIDs[j])
			}
			hash := sha256.Sum256([]byte(doc.XML))
			batch.pkg.Documents[j] = domain.BatchInvoiceDocument{
				Cuf: doc.Cuf, Xml: doc.XML, XmlHash: fmt.Sprintf("%x", hash),
				Archivo: doc.Archivo, HashArchivo: doc.HashArchivo,
			}
		}
	}
	return nil
}
