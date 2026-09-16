package usecase

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

// Los documentos fiscales se construyen exclusivamente a partir de facturas
// persistidas. El consumidor solo elige el POS y, opcionalmente, sus facturas.
func (uc *SiatUsecase) EnviarMasiva(ctx context.Context, companyID, posID string, body MasivaInput) (*PaqueteResultado, error) {
	return uc.sendInvoiceBatches(ctx, companyID, posID, body.FacturaIDs, domain.PackageTypeMasiva)
}

func (uc *SiatUsecase) EnviarPaquete(ctx context.Context, companyID, posID string, body PaqueteInput) (*PaqueteResultado, error) {
	return uc.sendInvoiceBatches(ctx, companyID, posID, body.FacturaIDs, domain.PackageTypePaquete)
}

type invoiceBatch struct {
	pkg       *domain.SentPackage
	documents []ports.FiscalDocument
	bulk      ports.FiscalBulk
	pack      ports.FiscalPackage
}

func (uc *SiatUsecase) batchRepository() (domain.FiscalBatchRepository, error) {
	repo, ok := uc.sentPackageRepo.(domain.FiscalBatchRepository)
	if !ok {
		return nil, domain.NewConflictError("La persistencia de lotes no está configurada")
	}
	return repo, nil
}

func (uc *SiatUsecase) batchEvent(posID string) (*domain.ContingencyEvent, error) {
	if uc.contingencyRepo == nil {
		return nil, domain.NewConflictError("No hay un evento de contingencia registrado")
	}
	event, err := uc.contingencyRepo.GetLatestByPointOfSale(posID)
	if err != nil {
		return nil, fmt.Errorf("obtener evento de contingencia: %w", err)
	}
	if event == nil || event.PointOfSaleID != posID || !event.IsSynced || event.EndDate == nil || event.SiatEventCode == nil {
		return nil, domain.NewConflictError("Registre y cierre el evento de contingencia antes de enviar sus facturas")
	}
	code, err := strconv.ParseInt(*event.SiatEventCode, 10, 64)
	if err != nil || code <= 0 || !event.EndDate.After(event.StartDate) {
		return nil, domain.NewConflictError("El evento de contingencia guardado no tiene una recepción o intervalo válido")
	}
	return event, nil
}

func (uc *SiatUsecase) selectBatchInvoices(repo domain.FiscalBatchRepository, companyID, posID string, ids []string, status domain.InvoiceStatus, event *domain.ContingencyEvent) ([]*domain.Invoice, error) {
	var invoices []*domain.Invoice
	var err error
	if ids == nil {
		var eventID *string
		if event != nil {
			eventID = &event.ID
		}
		invoices, err = repo.ListPendingBatchInvoices(companyID, posID, status, eventID)
	} else {
		if len(ids) == 0 {
			return nil, domain.NewBadRequestError("invoice_ids debe contener al menos una factura cuando se proporciona")
		}
		seen := make(map[string]bool, len(ids))
		for _, id := range ids {
			if strings.TrimSpace(id) == "" || seen[id] {
				return nil, domain.NewBadRequestError("invoice_ids contiene identificadores vacíos o duplicados")
			}
			seen[id] = true
		}
		invoices, err = uc.invoiceRepo.GetByIDs(ids)
		if err == nil && len(invoices) != len(ids) {
			return nil, domain.NewNotFoundError("No se encontraron todas las facturas seleccionadas")
		}
	}
	if err != nil {
		return nil, fmt.Errorf("obtener facturas del lote: %w", err)
	}
	for _, inv := range invoices {
		if inv == nil || inv.CompanyId != companyID || inv.PointOfSaleId != posID {
			return nil, domain.NewNotFoundError("La selección contiene facturas ajenas al punto de venta")
		}
		if inv.Status != status || (inv.SiatReceptionCode != nil && *inv.SiatReceptionCode != "") {
			return nil, domain.NewConflictError("La factura " + inv.ID + " ya fue enviada o no está pendiente para este tipo de lote")
		}
		if event != nil {
			if inv.ContingencyEventId == nil || *inv.ContingencyEventId != event.ID {
				return nil, domain.NewConflictError("La factura " + inv.ID + " no pertenece al evento de contingencia")
			}
			if inv.IssueDate.Before(event.StartDate) || inv.IssueDate.After(*event.EndDate) {
				return nil, domain.NewConflictError("La fecha de la factura " + inv.ID + " está fuera del evento de contingencia")
			}
		} else {
			if inv.ContingencyEventId != nil || inv.EmissionType == "OFFLINE" || inv.EmissionType == "MASIVA" || (inv.Cuf != nil && *inv.Cuf != "") {
				return nil, domain.NewConflictError("La factura " + inv.ID + " ya tiene una identidad fiscal o pertenece a una contingencia; no puede emitirse como masiva")
			}
		}
	}
	// Orden estable para dividir lotes y para asociar CUFs con las facturas.
	sort.Slice(invoices, func(i, j int) bool {
		if invoices[i].InvoiceNumber != invoices[j].InvoiceNumber {
			return invoices[i].InvoiceNumber < invoices[j].InvoiceNumber
		}
		return invoices[i].ID < invoices[j].ID
	})
	return invoices, nil
}

func (uc *SiatUsecase) prepareInvoiceBatches(company *domain.Company, pos *domain.PointOfSale, current *domain.Cufd, invoices []*domain.Invoice, event *domain.ContingencyEvent, kind domain.SentPackageType) ([]invoiceBatch, error) {
	if current == nil || current.ID == "" || current.PointOfSaleID != pos.ID || pos.Cuis == nil || *pos.Cuis == "" {
		return nil, domain.NewConflictError("No se pudieron resolver las credenciales del punto de venta")
	}
	limit, emission := siat.MaxFacturasMasiva, siat.EmisionMasiva
	if kind == domain.PackageTypePaquete {
		limit, emission = siat.MaxFacturasPorPaquete, siat.EmisionPaqueteOffline
	}
	type groupKey struct {
		sector, invoiceType, modality int
		layout, cufdID                string
	}
	groups := make(map[groupKey]int)
	batches := make([]invoiceBatch, 0)
	for _, inv := range invoices {
		credential := current
		if kind == domain.PackageTypePaquete {
			credential = &inv.CufdRecord
			if inv.Xml == nil || strings.TrimSpace(*inv.Xml) == "" || inv.Cuf == nil || *inv.Cuf == "" {
				return nil, domain.NewConflictError("La factura offline " + inv.ID + " no conserva su XML y CUF originales")
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
		profile, err := siat.PerfilSectorLayout(inv.CodigoDocumentoSector, inv.Layout)
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
				FechaEmision: inv.IssueDate, XML: *inv.Xml, Cuf: *inv.Cuf,
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
		if doc.Modalidad != siat.ModalidadElectronica && doc.Modalidad != siat.ModalidadComputarizada {
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
			doc.XML, doc.Cuf = *inv.Xml, *inv.Cuf
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

func fiscalBatchRequest(pkg *domain.SentPackage, company *domain.Company, pos *domain.PointOfSale) ports.FiscalBulk {
	return ports.FiscalBulk{
		CodigoAmbiente: company.Ambiente.CodigoAmbiente(), Nit: company.Nit,
		Modalidad: pkg.Modalidad, CodigoSucursal: pos.CodigoSucursal, CodigoPuntoVenta: resolveCodigoPuntoVenta(pos),
		Cuis: pkg.Cuis, Cufd: pkg.Cufd, CodigoDocumentoSector: pkg.CodigoDocumentoSector,
		CodigoTipoFactura: pkg.CodigoTipoFactura, CodigoEmision: pkg.CodigoEmision, Layout: pkg.Layout,
	}
}

func packageRequest(bulk ports.FiscalBulk, eventCode *int64) ports.FiscalPackage {
	pkg := ports.FiscalPackage{
		CodigoAmbiente: bulk.CodigoAmbiente, Nit: bulk.Nit, Modalidad: bulk.Modalidad,
		CodigoSucursal: bulk.CodigoSucursal, CodigoPuntoVenta: bulk.CodigoPuntoVenta,
		Cuis: bulk.Cuis, Cufd: bulk.Cufd, CodigoControl: bulk.CodigoControl,
		CodigoDocumentoSector: bulk.CodigoDocumentoSector, CodigoTipoFactura: bulk.CodigoTipoFactura,
		CodigoEmision: bulk.CodigoEmision, Layout: bulk.Layout, Facturas: bulk.Facturas,
	}
	if eventCode != nil {
		pkg.CodigoEvento = *eventCode
	}
	return pkg
}

// Preparar todos los lotes antes de reservar evita que un error local en una
// factura tardía deje envíos parciales o facturas atascadas en SENDING.
func prepareBatchPayloads(ctx context.Context, svc ports.FiscalService, batches []invoiceBatch, company *domain.Company, pos *domain.PointOfSale, current *domain.Cufd) error {
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

func (uc *SiatUsecase) sendInvoiceBatches(ctx context.Context, companyID, posID string, ids []string, kind domain.SentPackageType) (*PaqueteResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if uc.invoiceRepo == nil {
		return nil, domain.NewConflictError("El repositorio de facturas no está configurado")
	}
	repo, err := uc.batchRepository()
	if err != nil {
		return nil, err
	}
	company, pos, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	status := domain.InvoicePending
	var event *domain.ContingencyEvent
	if kind == domain.PackageTypePaquete {
		status = domain.InvoiceOffline
		event, err = uc.batchEvent(pos.ID)
		if err != nil {
			return nil, err
		}
	}
	invoices, err := uc.selectBatchInvoices(repo, companyID, posID, ids, status, event)
	if err != nil {
		return nil, err
	}
	out := &PaqueteResultado{Company: company, PointOfSale: pos, Batches: []BatchResultado{}, Response: &ports.FiscalPackageResult{Transaccion: true}}
	if len(invoices) == 0 {
		return out, nil
	}
	if uc.credentials == nil {
		return nil, domain.NewConflictError("El servicio de credenciales no está configurado")
	}
	if err := uc.credentials.EnsureCuis(ctx, company, pos); err != nil {
		return nil, err
	}
	current, err := uc.credentials.EnsureCufd(ctx, company, pos)
	if err != nil {
		return nil, err
	}
	batches, err := uc.prepareInvoiceBatches(company, pos, current, invoices, event, kind)
	if err != nil {
		return nil, err
	}
	svc, err := uc.resolveService(ctx, companyID)
	if err != nil {
		return nil, err
	}
	if err := prepareBatchPayloads(ctx, svc, batches, company, pos, current); err != nil {
		return nil, err
	}
	for _, batch := range batches {
		pkg := batch.pkg
		item := BatchResultado{InvoiceIDs: pkg.InvoiceIDs, Status: "NOT_SENT"}
		if err := ctx.Err(); err != nil {
			item.Error = err.Error()
			out.Batches = append(out.Batches, item)
			out.Response.Transaccion = false
			continue
		}
		if err := repo.ReserveBatch(pkg, pkg.InvoiceIDs, status); err != nil {
			item.Error = err.Error()
			out.Batches = append(out.Batches, item)
			out.Response.Transaccion = false
			continue
		}
		item.BatchID = pkg.ID
		var result ports.FiscalPackageResult
		if kind == domain.PackageTypeMasiva {
			result, err = svc.SendBulk(ctx, batch.bulk)
		} else {
			result, err = svc.SendPackage(ctx, batch.pack)
		}
		var invoiceStatus *domain.InvoiceStatus
		if err != nil {
			// Una respuesta perdida no demuestra que SIAT no recibió el lote.
			// Conservar la reserva evita una segunda emisión accidental.
			pkg.Status, item.Error = domain.PackageStatusUnknown, err.Error()
			pkg.Mensajes = &item.Error
			out.Response.Transaccion = false
		} else {
			item.Response = &result
			pkg.CodigoRecepcion = result.CodigoRecepcion
			messages, _ := json.Marshal(result.Mensajes)
			text := string(messages)
			pkg.Mensajes = &text
			if result.Transaccion && result.CodigoRecepcion != "" {
				pkg.Status = domain.PackageStatusPending
				sent := domain.InvoiceSent
				if result.CodigoEstado == 908 {
					pkg.Status, sent = domain.PackageStatusAccepted, domain.InvoiceAccepted
				}
				invoiceStatus = &sent
				out.Response.CantidadFacturas += pkg.CantidadFacturas
			} else if result.CodigoEstado == 902 {
				pkg.Status = domain.PackageStatusRejected
				rejected := domain.InvoiceRejected
				invoiceStatus = &rejected
				out.Response.Transaccion = false
			} else {
				pkg.Status = domain.PackageStatusUnknown
				out.Response.Transaccion = false
			}
		}
		if persistErr := repo.UpdateBatch(pkg, invoiceStatus); persistErr != nil {
			item.Error = strings.TrimSpace(fmt.Sprintf("%s No se pudo guardar el resultado del lote %s: %v", item.Error, pkg.ID, persistErr))
			// El estado de SIAT puede conocerse, pero la BD conserva la reserva.
			pkg.Status = domain.PackageStatusUnknown
			out.Response.Transaccion = false
		}
		item.Status = pkg.Status
		out.Batches = append(out.Batches, item)
	}
	return out, nil
}

func (uc *SiatUsecase) ValidarMasiva(ctx context.Context, companyID, posID string, body PaqueteValidacionInput) (*PaqueteResultado, error) {
	return uc.validateInvoiceBatch(ctx, companyID, posID, body.BatchID, domain.PackageTypeMasiva)
}

func (uc *SiatUsecase) ValidarPaquete(ctx context.Context, companyID, posID string, body PaqueteValidacionInput) (*PaqueteResultado, error) {
	return uc.validateInvoiceBatch(ctx, companyID, posID, body.BatchID, domain.PackageTypePaquete)
}

func (uc *SiatUsecase) validateInvoiceBatch(ctx context.Context, companyID, posID, batchID string, kind domain.SentPackageType) (*PaqueteResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	repo, err := uc.batchRepository()
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(batchID) == "" {
		return nil, domain.NewBadRequestError("El identificador del lote es obligatorio")
	}
	pkg, err := uc.sentPackageRepo.GetByID(batchID)
	if err != nil {
		return nil, err
	}
	if pkg == nil || pkg.CompanyId != companyID || pkg.Type != kind || (posID != "" && pkg.PointOfSaleId != posID) {
		return nil, domain.NewNotFoundError("Lote no encontrado")
	}
	if pkg.CodigoRecepcion == "" || pkg.CodigoDocumentoSector <= 0 || pkg.CodigoTipoFactura <= 0 || pkg.Modalidad <= 0 || pkg.Cufd == "" || pkg.Cuis == "" {
		return nil, domain.NewConflictError("El lote no tiene recepción o metadatos completos para validarlo")
	}
	company, pos, err := uc.LoadCompanyAndPointOfSale(companyID, pkg.PointOfSaleId)
	if err != nil {
		return nil, err
	}
	svc, err := uc.resolveService(ctx, companyID)
	if err != nil {
		return nil, err
	}
	req := fiscalBatchRequest(pkg, company, pos)
	var result ports.FiscalPackageResult
	if kind == domain.PackageTypeMasiva {
		result, err = svc.ValidateBulk(ctx, req, pkg.CodigoRecepcion)
	} else {
		result, err = svc.ValidatePackage(ctx, packageRequest(req, pkg.CodigoEvento), pkg.CodigoRecepcion)
	}
	if err != nil {
		return nil, err
	}
	var invoiceStatus *domain.InvoiceStatus
	// Transaccion solo confirma la consulta. 908 confirma la validación fiscal;
	// una recepción observada puede contener facturas con resultados distintos.
	if result.Transaccion && result.CodigoEstado == 908 {
		pkg.Status = domain.PackageStatusAccepted
		accepted := domain.InvoiceAccepted
		invoiceStatus = &accepted
	} else if result.CodigoEstado == 902 && pkg.Status != domain.PackageStatusAccepted {
		pkg.Status = domain.PackageStatusRejected
		rejected := domain.InvoiceRejected
		invoiceStatus = &rejected
	}
	now := time.Now()
	pkg.ValidatedAt = &now
	messages, _ := json.Marshal(result.Mensajes)
	text := string(messages)
	pkg.Mensajes = &text
	if err := repo.UpdateBatch(pkg, invoiceStatus); err != nil {
		return nil, fmt.Errorf("guardar validación del lote %s: %w", pkg.ID, err)
	}
	return &PaqueteResultado{Company: company, PointOfSale: pos, Response: &result,
		Batches: []BatchResultado{{BatchID: pkg.ID, InvoiceIDs: pkg.InvoiceIDs, Status: pkg.Status, Response: &result}}}, nil
}
