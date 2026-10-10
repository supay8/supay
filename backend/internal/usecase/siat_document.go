package usecase

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

func (uc *SiatUsecase) solicitudDesdeInvoice(inv *domain.Invoice, company *domain.Company, pos *domain.PointOfSale, cufd *domain.Cufd) (ports.FiscalDocument, error) {
	// Resuelve leyenda como en InvoiceUsecase.resolveLeyenda
	actividad := ""
	if company.CodigoActividad != nil {
		actividad = strings.TrimSpace(*company.CodigoActividad)
	}
	leyenda := "Ley N° 453: Tienes derecho a recibir información sobre el Sistema de Facturación, Ley N° 453, de 4 de diciembre de 2013."
	if uc.leyendaRepo != nil && actividad != "" {
		if items, err := uc.leyendaRepo.ListByActividad(company.ID, actividad); err == nil {
			for _, it := range items {
				if strings.TrimSpace(it.DescripcionLeyenda) != "" {
					leyenda = strings.TrimSpace(it.DescripcionLeyenda)
					break
				}
			}
		}
	}
	// El bloque de cliente del SIAT se construye desde el snapshot de la factura.
	cliente, err := clienteFromCustomer(inv.Customer)
	if err != nil {
		return ports.FiscalDocument{}, err
	}
	// Dirección padrón
	direccion := strings.TrimSpace(cufd.Direccion)
	if direccion == "" {
		direccion = company.Direccion
	}
	var telPtr *string
	if strings.TrimSpace(company.Telefono) != "" {
		t := strings.TrimSpace(company.Telefono)
		telPtr = &t
	}
	usuario := "SUPAY"
	if company.UsuarioSiat != "" {
		usuario = company.UsuarioSiat
	}
	// Items mapeados desde InvoiceItem ya persistidos
	solItems := make([]ports.FiscalItem, 0, len(inv.Items))
	for _, it := range inv.Items {
		var codigoSin int64
		if it.CodigoProductoSin != nil {
			if parsed, err := strconv.ParseInt(strings.TrimSpace(*it.CodigoProductoSin), 10, 64); err == nil {
				codigoSin = parsed
			}
		}
		if codigoSin <= 0 {
			return ports.FiscalDocument{}, domain.NewBadRequestError("el ítem " + it.Description + " no tiene codigo_producto_sin válido")
		}
		act := actividad
		if it.CodigoActividad != nil && strings.TrimSpace(*it.CodigoActividad) != "" {
			act = strings.TrimSpace(*it.CodigoActividad)
		}
		unidad := 1
		if it.UnitCode != nil && *it.UnitCode > 0 {
			unidad = *it.UnitCode
		}
		var discPtr *float64
		if it.Discount != 0 {
			d := it.Discount
			discPtr = &d
		}
		solItems = append(solItems, ports.FiscalItem{
			ActividadEconomica: act,
			CodigoProductoSin:  codigoSin,
			CodigoProducto:     it.Code,
			Descripcion:        it.Description,
			Cantidad:           it.Quantity,
			UnidadMedida:       unidad,
			PrecioUnitario:     it.UnitPrice,
			MontoDescuento:     discPtr,
			SubTotal:           it.Subtotal,
			DatosSector:        it.SectorData,
		})
	}
	tipoFactura := inv.CodigoTipoFactura
	if tipoFactura <= 0 {
		if perfil, err := fiscal.PerfilSectorLayout(inv.CodigoDocumentoSector, inv.Layout); err == nil {
			tipoFactura = perfil.TipoDocumentoResuelto(0)
		} else {
			tipoFactura = 1
		}
	}
	return ports.FiscalDocument{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         "",
		Nit:                   company.Nit,
		Modalidad:             inv.Modalidad,
		NumeroFactura:         int64(inv.InvoiceNumber),
		CodigoSucursal:        pos.CodigoSucursal,
		CodigoPuntoVenta:      pos.CodigoPuntoVenta,
		Cuis:                  "", // se hereda del paquete
		Cufd:                  "",
		CodigoControl:         "",
		FechaEmision:          inv.IssueDate,
		Usuario:               usuario,
		Leyenda:               leyenda,
		RazonSocialEmisor:     company.BusinessName,
		Municipio:             company.Municipio,
		Direccion:             direccion,
		Telefono:              telPtr,
		CodigoMetodoPago:      inv.CodigoMetodoPago,
		CodigoMoneda:          inv.CodigoMoneda,
		TipoCambio:            inv.TipoCambio,
		MontoTotal:            inv.Total,
		CodigoDocumentoSector: inv.CodigoDocumentoSector,
		Layout:                inv.Layout,
		CodigoTipoFactura:     tipoFactura,
		DatosSector:           inv.SectorData,
		Archivo:               inv.Archivo,
		HashArchivo:           inv.HashArchivo,
		Cuf:                   "",
		Cliente:               cliente,
		Items:                 solItems,
	}, nil
}

// ResolveDocumentoSector determina el documento-sector del SIAT para la
// actividad económica de la empresa consultando el catálogo sincronizado
// actividadesDocumentoSector (tabla siat_actividades_doc_sector). Prefiere la
// factura de compraventa (FCV); si la actividad solo está asociada a sectores
// educativos (FSEDU), usa ese sector.
func (uc *SiatUsecase) ResolveDocumentoSector(company *domain.Company) (int, error) {
	if company == nil {
		return 0, errors.New("no se puede resolver documento-sector sin empresa")
	}
	if uc.docSectorRepo == nil {
		return 0, errors.New("no existe repositorio de actividadesDocumentoSector; sincronice la lista de actividades")
	}
	if company.CodigoActividad == nil {
		return 0, fmt.Errorf("la empresa %s no tiene codigo_actividad; sincronice actividadesDocumentoSector", company.ID)
	}
	actividad := strings.TrimSpace(*company.CodigoActividad)
	if actividad == "" {
		return 0, fmt.Errorf("la empresa %s no tiene codigo_actividad; sincronice actividadesDocumentoSector", company.ID)
	}
	items, err := uc.docSectorRepo.ListByActividad(company.ID, actividad)
	if err != nil {
		return 0, fmt.Errorf("no se pudo resolver documento-sector para actividad %s: %w", actividad, err)
	}
	if len(items) == 0 {
		return 0, fmt.Errorf("actividad %s no sincronizada en actividadesDocumentoSector; ejecute la sincronizacion antes de emitir", actividad)
	}
	found := 0
	for _, item := range items {
		switch strings.TrimSpace(item.TipoDocumentoSector) {
		case "FCV":
			if item.CodigoDocumentoSector > 0 {
				return item.CodigoDocumentoSector, nil
			}
		case "FSEDU":
			found = item.CodigoDocumentoSector
		}
	}
	if found > 0 {
		return found, nil
	}
	return 0, fmt.Errorf("actividad %s no tiene una relacion de documento-sector soportada; sincronice nuevamente el catalogo", actividad)
}
