package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

// buildSolicitudDocumento reúne la identificación del documento ya emitido para
// las operaciones de verificación, anulación y reversión de anulación. El CUFD
// que se envía es el VIGENTE del punto de venta (GetActiveByPos), porque el SIAT
// rechaza operaciones firmadas con un CUFD vencido (vigencia ~24h); si no hay
// CUFD vigente registrado se cae al CUFD con el que se emitió la factura.
func (uc *InvoiceUsecase) buildSolicitudDocumento(inv *domain.Invoice) (*ports.FiscalDocumentQuery, error) {
	pos := inv.PointOfSale
	if pos.Cuis == nil || strings.TrimSpace(*pos.Cuis) == "" {
		return nil, domain.NewConflictError("el punto de venta no tiene cuis activo")
	}

	// Obtener CUFD vigente del punto de venta, o caer al registrado con la factura.
	var cufd *domain.Cufd
	if uc.cufdRepo != nil {
		if active, err := uc.cufdRepo.GetActiveByPos(inv.PointOfSaleId); err == nil && active != nil {
			cufd = active
		}
	}
	if cufd == nil {
		if inv.CufdRecord.ID != "" && strings.TrimSpace(inv.CufdRecord.Cufd) != "" {
			cufd = &inv.CufdRecord
		}
	}
	if cufd == nil || strings.TrimSpace(cufd.Cufd) == "" {
		return nil, domain.NewConflictError("la factura no tiene un cufd vigente asociado; solicítelo primero")
	}

	// Validar vigencia del CUFD: el SIAT rechaza operaciones con CUFD vencido.
	now := time.Now().In(fiscal.LaPaz)
	if now.Before(cufd.ValidFrom) || now.After(cufd.ValidTo) {
		return nil, fmt.Errorf("el CUFD está vencido (válido desde %s hasta %s); solicite uno nuevo (POST /siat/cufd/...)",
			cufd.ValidFrom.In(fiscal.LaPaz).Format("2006-01-02 15:04"),
			cufd.ValidTo.In(fiscal.LaPaz).Format("2006-01-02 15:04"))
	}

	codigoPuntoVenta := pos.CodigoPuntoVenta
	if pos.SiatCode != nil {
		codigoPuntoVenta = *pos.SiatCode
	}

	modalidad := inv.Modalidad
	if modalidad <= 0 {
		modalidad = uc.effectiveModalidadForCompany(&inv.Company)
	}
	if modalidad <= 0 {
		modalidad = fiscal.ModalidadElectronica
	}

	return &ports.FiscalDocumentQuery{
		CodigoAmbiente:        inv.Company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         "",
		Nit:                   inv.Company.Nit,
		Modalidad:             modalidad,
		Layout:                inv.Layout,
		Cuf:                   *inv.Cuf,
		CodigoSucursal:        pos.CodigoSucursal,
		CodigoPuntoVenta:      codigoPuntoVenta,
		Cuis:                  *pos.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoDocumentoSector: inv.CodigoDocumentoSector,
		CodigoTipoFactura:     inv.CodigoTipoFactura,
	}, nil
}

// buildSolicitudFactura reúne los prerrequisitos de la factura y los mapea a
// los códigos de catálogo SIN esperados por el SDK.
func (uc *InvoiceUsecase) buildSolicitudFactura(ctx context.Context, inv *domain.Invoice) (*ports.FiscalDocument, error) {
	company := inv.Company
	pos := inv.PointOfSale

	// CUIS lazy: si falta y hay servicio de credenciales, se solicita en
	// línea; si no, se mantiene el error accionable.
	if pos.Cuis == nil || strings.TrimSpace(*pos.Cuis) == "" {
		if uc.credentials != nil {
			if err := uc.credentials.EnsureCuis(ctx, &company, &pos); err != nil {
				return nil, err
			}
		}
	}
	if pos.Cuis == nil || strings.TrimSpace(*pos.Cuis) == "" {
		return nil, domain.NewConflictError("el punto de venta no tiene cuis activo; solicítelo primero")
	}

	// CUFD: con credenciales se resuelve lazy (renueva si venció); sin
	// ellas se usa el vigente del punto de venta o el de la factura.
	var cufd domain.Cufd
	if inv.Archivo != "" {
		cufd = inv.CufdRecord
	} else if uc.credentials != nil {
		ensured, err := uc.credentials.EnsureCufd(ctx, &company, &pos)
		if err != nil {
			return nil, err
		}
		cufd = *ensured
	} else {
		cufd = inv.CufdRecord
		if uc.cufdRepo != nil {
			if active, err := uc.cufdRepo.GetActiveByPos(inv.PointOfSaleId); err == nil && active != nil {
				cufd = *active
			}
		}
		if cufd.ID == "" || !cufd.Active {
			return nil, domain.NewConflictError("la factura no tiene un cufd vigente asociado; solicítelo primero")
		}
		now := time.Now().In(fiscal.LaPaz)
		if now.Before(cufd.ValidFrom) || now.After(cufd.ValidTo) {
			return nil, domain.NewConflictError("el cufd asociado a la factura está vencido; solicite uno nuevo")
		}
	}
	if cufd.ID != "" {
		inv.CufdId = cufd.ID
		inv.CufdRecord = cufd
	}

	if company.CodigoActividad == nil || strings.TrimSpace(*company.CodigoActividad) == "" {
		return nil, domain.NewConflictError("la empresa no tiene definida su actividad económica (codigo_actividad)")
	}
	actividad := strings.TrimSpace(*company.CodigoActividad)

	// El código de punto de venta que usa el CUFD/CUIS debe ser el mismo que
	// viaja en el CUF y en la recepción (preferir el código registrado ante SIAT).
	codigoPuntoVenta := pos.CodigoPuntoVenta
	if pos.SiatCode != nil {
		codigoPuntoVenta = *pos.SiatCode
	}

	modalidad := inv.Modalidad
	if modalidad <= 0 {
		modalidad = uc.effectiveModalidadForCompany(&company)
	}
	if modalidad <= 0 {
		modalidad = fiscal.ModalidadElectronica
	}

	leyenda, err := uc.resolveLeyenda(inv.CompanyId, actividad)
	if err != nil {
		return nil, err
	}

	telefono := strings.TrimSpace(company.Telefono)
	if telefono == "" {
		telefono = "0000000"
	}
	telefonoPtr := &telefono

	habilitadas := uc.actividadesHabilitadas(&company)
	items := make([]ports.FiscalItem, 0, len(inv.Items))
	for i, it := range inv.Items {
		itemActividad := actividad
		if it.CodigoActividad != nil && strings.TrimSpace(*it.CodigoActividad) != "" {
			itemActividad = strings.TrimSpace(*it.CodigoActividad)
		} else {
			// Herencia por defecto de actividad principal si no viene definida
			itemActividad = actividad
		}
		// Multiactividad controlada: validar existencia en habilitadas (Warn, no bloquea)
		if len(habilitadas) > 0 && !habilitadas[itemActividad] {
			slog.Warn("emission: actividad item no habilitada en padrón, se emite con advertencia (evitar 1017)",
				"invoice_id", inv.ID, "item", i+1, "actividad_item", itemActividad, "actividad_principal", actividad, "habilitadas", habilitadas)
		}

		var codigoProductoSin int64
		if it.CodigoProductoSin != nil {
			if parsed, perr := strconv.ParseInt(strings.TrimSpace(*it.CodigoProductoSin), 10, 64); perr == nil && parsed > 0 {
				codigoProductoSin = parsed
			}
		}
		if codigoProductoSin <= 0 {
			return nil, domain.NewBadRequestError(fmt.Sprintf("el ítem %d (%s) no tiene un codigo_producto_sin válido; sincronice el catálogo y asigne el código SIN", i+1, it.Description))
		}

		unidadMedida := 1
		if it.UnitCode != nil && *it.UnitCode > 0 {
			unidadMedida = *it.UnitCode
		}

		descuento := it.Discount
		var descuentoPtr *float64
		if descuento > 0 {
			descuentoPtr = &descuento
		}

		// Subtotal dinámico estricto: corrige valores quemados/desalineados (1013/1018)
		subtotalCalc := fiscal.CalcularSubtotal(it.Quantity, it.UnitPrice, descuentoPtr)
		if it.Subtotal != 0 && round2(it.Subtotal) != subtotalCalc {
			slog.Warn("emission: subtotal item auto-corregido",
				"invoice_id", inv.ID, "item", i+1, "descripcion", it.Description,
				"subtotal_previo", it.Subtotal, "subtotal_corregido", subtotalCalc)
		}
		items = append(items, ports.FiscalItem{
			ActividadEconomica: itemActividad,
			CodigoProductoSin:  codigoProductoSin,
			CodigoProducto:     it.Code,
			Descripcion:        it.Description,
			Cantidad:           it.Quantity,
			UnidadMedida:       unidadMedida,
			PrecioUnitario:     it.UnitPrice,
			MontoDescuento:     descuentoPtr,
			SubTotal:           subtotalCalc,
			DatosSector:        it.SectorData,
		})
	}
	// MontoTotal dinámico: suma estricta de subtotales (auto-corrección con Warn)
	var montoCorregido float64
	for _, it := range items {
		montoCorregido += it.SubTotal
	}
	montoCorregido = round2(montoCorregido)
	if inv.CodigoDocumentoSector == 30 && len(items) == 0 {
		montoCorregido = inv.Subtotal
	}
	montoCorregido, err = fiscal.TotalDocumento(montoCorregido, inv.SectorData)
	if err != nil {
		return nil, domain.NewBadRequestError(err.Error())
	}
	if round2(inv.Total) != montoCorregido {
		slog.Warn("emission: montoTotal auto-corregido",
			"invoice_id", inv.ID, "monto_previo", inv.Total, "monto_corregido", montoCorregido)
	}

	// La dirección del XML debe coincidir con la registrada en padrón ante el
	// SIAT (la trae el CUFD); si no, la factura se observa (código 1007).
	direccion := strings.TrimSpace(cufd.Direccion)
	if direccion == "" {
		direccion = company.Direccion
	}

	sector := inv.CodigoDocumentoSector
	if sector <= 0 {
		sector = 1
	}
	perfil, err := fiscal.PerfilSectorLayout(sector, inv.Layout)
	if err != nil {
		return nil, domain.NewBadRequestError(fmt.Sprintf("factura %s: %v", inv.ID, err))
	}
	var numeroFacturaOriginal int64
	var originalItems []ports.FiscalItem
	if perfil.EsAjuste() {
		if strings.TrimSpace(valueOrEmpty(inv.AjustaFacturaId)) == "" {
			return nil, domain.NewConflictError("el documento de ajuste no tiene factura original asociada")
		}
		original, originalErr := uc.invoiceRepo.GetByID(inv.CompanyId, *inv.AjustaFacturaId)
		if originalErr != nil {
			return nil, fmt.Errorf("no se pudo cargar la factura original del ajuste: %w", originalErr)
		}
		slog.Info("siat ajuste: factura original cargada",
			"invoice_id", inv.ID,
			"original_id", original.ID,
			"nota_invoice_number", inv.InvoiceNumber,
			"original_invoice_number", original.InvoiceNumber,
			"original_cuf", valueOrEmpty(original.Cuf),
			"layout", inv.Layout,
			"sector", sector)
		if original.InvoiceNumber <= 0 {
			return nil, domain.NewConflictError("la factura original del ajuste no tiene un número válido")
		}
		numeroFacturaOriginal = int64(original.InvoiceNumber)
		if perfil.DetallePar || perfil.Codigo == 29 || perfil.Codigo == fiscal.SectorNotaCreditoDebito {
			for _, item := range original.Items {
				if item.CodigoProductoSin == nil || item.CodigoActividad == nil || item.UnitCode == nil {
					return nil, domain.NewConflictError("la factura original no tiene datos fiscales completos en sus ítems")
				}
				code, err := strconv.ParseInt(*item.CodigoProductoSin, 10, 64)
				if err != nil || code <= 0 {
					return nil, domain.NewConflictError("código SIN inválido en factura original")
				}
				data, err := datosDetalleOriginal(perfil, item.SectorData)
				if err != nil {
					return nil, err
				}
				discount := item.Discount
				originalItems = append(originalItems, ports.FiscalItem{
					ActividadEconomica: *item.CodigoActividad, CodigoProductoSin: code,
					CodigoProducto: item.Code, Descripcion: item.Description,
					Cantidad: item.Quantity, PrecioUnitario: item.UnitPrice, UnidadMedida: *item.UnitCode,
					MontoDescuento: &discount, SubTotal: item.Subtotal, DatosSector: data,
				})
			}
		}
	}
	tipoFactura := perfil.TipoDocumentoResuelto(inv.CodigoTipoFactura)

	// Los campos legados educativos viajan siempre; el paquete siat los ignora
	// fuera de los sectores 11/46 y los fusiona a datos_sector cuando aplica.
	var nombreEstudiante, periodoFacturado string
	if inv.NombreEstudiante != nil {
		nombreEstudiante = strings.TrimSpace(*inv.NombreEstudiante)
	}
	if inv.PeriodoFacturado != nil {
		periodoFacturado = strings.TrimSpace(*inv.PeriodoFacturado)
	}

	usuario := "SUPAY"
	if company.UsuarioSiat != "" {
		usuario = company.UsuarioSiat
	}

	// El bloque de cliente del SIAT se construye desde el snapshot de la factura.
	cliente, err := clienteFromCustomer(inv.Customer)
	if err != nil {
		return nil, err
	}

	return &ports.FiscalDocument{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         "",
		Nit:                   company.Nit,
		Modalidad:             modalidad,
		NumeroFactura:         int64(inv.InvoiceNumber),
		NumeroFacturaOriginal: numeroFacturaOriginal,
		CodigoSucursal:        pos.CodigoSucursal,
		CodigoPuntoVenta:      codigoPuntoVenta,
		Cuis:                  *pos.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoControl:         cufd.ControlCode,
		FechaEmision:          inv.IssueDate,
		Usuario:               usuario,
		Leyenda:               leyenda,
		RazonSocialEmisor:     company.BusinessName,
		Municipio:             company.Municipio,
		Direccion:             direccion,
		Telefono:              telefonoPtr,
		CodigoMetodoPago:      inv.CodigoMetodoPago,
		CodigoMoneda:          inv.CodigoMoneda,
		TipoCambio:            inv.TipoCambio,
		MontoTotal:            montoCorregido,
		CodigoDocumentoSector: sector,
		Layout:                inv.Layout,
		CodigoTipoFactura:     tipoFactura,
		NombreEstudiante:      nombreEstudiante,
		PeriodoFacturado:      periodoFacturado,
		DatosSector:           inv.SectorData,
		Archivo:               inv.Archivo,
		HashArchivo:           inv.HashArchivo,
		Cuf:                   valueOrEmpty(inv.Cuf),
		Cliente:               cliente,
		Items:                 items,
		OriginalItems:         originalItems,
	}, nil
}

// clienteFromCustomer construye el bloque ClienteFactura del SIAT desde el
// snapshot embebido en la factura. Nunca consulta la dimensión customers.
func clienteFromCustomer(c domain.Customer) (ports.FiscalCustomer, error) {
	if strings.TrimSpace(c.DocumentNumber) == "" || strings.TrimSpace(c.Name) == "" {
		return ports.FiscalCustomer{}, domain.NewConflictError("factura sin cliente asociado; toda factura debe referenciar un cliente")
	}
	codigoDoc, err := codigoTipoDocumentoIdentidad(c.DocumentType)
	if err != nil {
		return ports.FiscalCustomer{}, err
	}
	complemento := c.Complement
	var codigoCliente *string
	if strings.TrimSpace(c.CodigoCliente) != "" {
		codigo := c.CodigoCliente
		codigoCliente = &codigo
		// SIAT XSD requiere que 'complemento' esté presente antes que
		// 'codigoCliente'. Si hay codigoCliente pero no complemento, se envía
		// string vacío (no nil) para mantener el orden del XSD y evitar el
		// rechazo 920.
		if complemento == nil {
			empty := ""
			complemento = &empty
		}
	}
	return ports.FiscalCustomer{
		NombreRazonSocial:            c.Name,
		CodigoTipoDocumentoIdentidad: codigoDoc,
		NumeroDocumento:              c.DocumentNumber,
		Complemento:                  complemento,
		CodigoCliente:                codigoCliente,
	}, nil
}

// codigoTipoDocumentoIdentidad mapea el tipo de documento del cliente al código
// del catálogo sincronizado tipoDocumentoIdentidad del SIN.
func codigoTipoDocumentoIdentidad(documentType string) (int, error) {
	switch strings.ToUpper(strings.TrimSpace(documentType)) {
	case "CI":
		return 1, nil
	case "CEX":
		return 2, nil
	case "PAS":
		return 3, nil
	case "NIT":
		return 4, nil
	case "OD":
		return 5, nil
	default:
		return 0, domain.NewBadRequestError(fmt.Sprintf("tipo de documento de identidad no soportado: %q", documentType))
	}
}

// resolveLeyenda busca en el catálogo sincronizado leyendasFactura (tabla
// siat_leyendas_factura) la leyenda oficial del SIAT para la actividad
// económica de la empresa. Si el catálogo no está sincronizado o no contiene
// la actividad, usa la leyenda genérica de la Ley 453.
func (uc *InvoiceUsecase) resolveLeyenda(companyID, actividad string) (string, error) {
	if uc.leyendaRepo != nil && strings.TrimSpace(actividad) != "" {
		if items, err := uc.leyendaRepo.ListByActividad(companyID, strings.TrimSpace(actividad)); err == nil {
			for _, item := range items {
				leyenda := strings.TrimSpace(item.DescripcionLeyenda)
				if leyenda != "" {
					return leyenda, nil
				}
			}
		}
	}
	return "Ley N° 453: Tienes derecho a recibir información sobre el Sistema de Facturación, Ley N° 453, de 4 de diciembre de 2013.", nil
}

func valueOrEmpty(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

// validateEmissionPayload validates sector fields before requesting CUIS/CUFD.
func (uc *InvoiceUsecase) validateEmissionPayload(inv *domain.Invoice) error {
	sector := inv.CodigoDocumentoSector
	if sector <= 0 {
		sector = 1
	}
	profile, err := fiscal.PerfilSectorLayout(sector, inv.Layout)
	if err != nil {
		return domain.NewBadRequestError(err.Error())
	}
	if err = profile.ValidarModalidad(uc.effectiveModalidadForCompany(&inv.Company)); err != nil {
		return domain.NewBadRequestError(err.Error())
	}
	_, err = profile.PrepararDatosSector(fiscal.SolicitudFactura{DatosSector: inv.SectorData, NombreEstudiante: valueOrEmpty(inv.NombreEstudiante), PeriodoFacturado: valueOrEmpty(inv.PeriodoFacturado)})
	if err != nil {
		return domain.NewBadRequestError(err.Error())
	}
	for i, item := range inv.Items {
		if _, err = profile.ValidarDatosDetalle(item.SectorData); err != nil {
			return domain.NewBadRequestError(fmt.Sprintf("ítem %d: %v", i+1, err))
		}
		var code int64
		if item.CodigoProductoSin != nil {
			code, _ = strconv.ParseInt(strings.TrimSpace(*item.CodigoProductoSin), 10, 64)
		}
		if code <= 0 {
			return domain.NewBadRequestError(fmt.Sprintf("el ítem %d (%s) no tiene un codigo_producto_sin válido; sincronice el catálogo y asigne el código SIN", i+1, item.Description))
		}
	}
	return nil
}
