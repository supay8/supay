package usecase

import (
	"context"
	"strings"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
)

// EnviarCompras registra en el SIAT un paquete de facturas de compras
// (recepcionPaqueteCompras, Etapa XI). codigoPuntoVenta no aplica en el
// servicio de compras.
func (uc *SiatUsecase) EnviarCompras(ctx context.Context, companyID, posID string, body ComprasInput) (*ComprasResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Archivo) == "" || strings.TrimSpace(body.HashArchivo) == "" {
		return nil, domain.NewBadRequestError("archivo y hashArchivo son obligatorios (Base64 del TAR.GZ y su SHA-256)")
	}

	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		return nil, domain.NewConflictError("El punto de venta no tiene CUIS activo")
	}
	cufd, err := uc.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		return nil, domain.NewConflictError("El punto de venta no tiene CUFD vigente")
	}

	req := ports.FiscalPurchase{
		Descripcion:      body.Descripcion,
		TipoCompra:       body.TipoCompra,
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    "",
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		CodigoPuntoVenta: 0,
		Cuis:             *pointOfSale.Cuis,
		Cufd:             cufd.Cufd,
		Archivo:          body.Archivo,
		HashArchivo:      body.HashArchivo,
		CantidadFacturas: body.CantidadFacturas,
		Gestion:          body.Gestion,
		Periodo:          body.Periodo,
		FechaEnvio:       body.FechaEnvio,
	}

	svc, err := uc.resolveService(ctx, company.ID)
	if err != nil {
		return nil, err
	}
	result, err := svc.SendPurchases(ctx, req)
	if err != nil {
		return nil, err
	}
	uc.persistSentPackage(company, pointOfSale, result.CodigoRecepcion, domain.PackageTypeCompras, 19, 0, body.CantidadFacturas)
	return &ComprasResultado{Company: company, PointOfSale: pointOfSale, Response: &result}, nil
}

// FirmarFactura firma digitalmente el XML de una factura con el certificado de
// la empresa (Etapa VIII - Firma Digital).
func (uc *SiatUsecase) FirmarFactura(ctx context.Context, companyID, posID string, body FirmaInput) (*FirmaResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.Xml) == "" {
		return nil, domain.NewBadRequestError("xml es obligatorio (la cadena del XML de la factura a firmar)")
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	svc, err := uc.resolveService(ctx, company.ID)
	if err != nil {
		return nil, err
	}
	result, err := svc.SignXML(ctx, ports.FiscalSignRequest{Xml: body.Xml})
	if err != nil {
		return nil, err
	}
	return &FirmaResultado{Company: company, PointOfSale: pointOfSale, Response: &result}, nil
}
