package usecase

import (
	"context"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

type DocumentoAjusteInput struct {
	NumeroFactura         int64                `json:"numeroFactura"`
	CufFacturaOriginal    string               `json:"cufFacturaOriginal"`
	CodigoDocumentoSector int                  `json:"codigoDocumentoSector"`
	Layout                string               `json:"layout,omitempty"`
	CodigoTipoFactura     int                  `json:"codigoTipoFactura"`
	TipoNota              int                  `json:"tipoNota"`
	Motivo                string               `json:"motivo"`
	CodigoMetodoPago      int                  `json:"codigoMetodoPago"`
	CodigoMoneda          int                  `json:"codigoMoneda"`
	TipoCambio            float64              `json:"tipoCambio"`
	MontoTotal            float64              `json:"montoTotal"`
	Leyenda               string               `json:"leyenda"`
	Cliente               ports.FiscalCustomer `json:"cliente"`
	Items                 []ports.FiscalItem   `json:"items"`
}

type DocumentoAjusteResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *ports.FiscalAdjustmentResult
}

// EmitirDocumentoAjuste emite un documento de ajuste (nota de crédito o
// débito) ante el SIAT. La identidad SIAT (CUIS/CUFD/código de control/
// modalidad) se resuelve SIEMPRE desde la base de datos — nunca del request —
// para impedir emitir NC/ND con un CUFD vencido o de otro punto de venta.
func (uc *SiatUsecase) EmitirDocumentoAjuste(ctx context.Context, companyID, posID string, body DocumentoAjusteInput) (*DocumentoAjusteResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(body.CufFacturaOriginal) == "" {
		return nil, domain.NewBadRequestError("cufFacturaOriginal es obligatorio para documentos de ajuste")
	}
	if body.TipoNota != 1 && body.TipoNota != 2 {
		return nil, domain.NewBadRequestError("tipoNota debe ser 1 (nota de crédito) o 2 (nota de débito)")
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

	usuario := "SUPAY"
	if company.UsuarioSiat != "" {
		usuario = company.UsuarioSiat
	}

	req := ports.FiscalAdjustment{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         "",
		Nit:                   company.Nit,
		Modalidad:             uc.effectiveModalidadForCompany(company),
		NumeroFactura:         body.NumeroFactura,
		CodigoSucursal:        pointOfSale.CodigoSucursal,
		CodigoPuntoVenta:      pointOfSale.CodigoPuntoVenta,
		Cuis:                  *pointOfSale.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoControl:         cufd.ControlCode,
		FechaEmision:          time.Now().In(fiscal.LaPaz),
		Usuario:               usuario,
		TipoNota:              body.TipoNota,
		CufFacturaOriginal:    body.CufFacturaOriginal,
		CodigoDocumentoSector: body.CodigoDocumentoSector,
		Layout:                body.Layout,
		CodigoTipoFactura:     body.CodigoTipoFactura,
		RazonSocialEmisor:     company.BusinessName,
		Municipio:             company.Municipio,
		Direccion:             company.Direccion,
		Telefono:              &company.Telefono,
		Cliente:               body.Cliente,
		CodigoMetodoPago:      body.CodigoMetodoPago,
		CodigoMoneda:          body.CodigoMoneda,
		TipoCambio:            body.TipoCambio,
		MontoTotal:            body.MontoTotal,
		Leyenda:               body.Leyenda,
		Motivo:                body.Motivo,
		Items:                 body.Items,
	}

	svc, err := uc.resolveService(ctx, company.ID)
	if err != nil {
		return nil, err
	}
	result, err := svc.EmitAdjustment(ctx, req)
	if err != nil {
		return nil, err
	}
	return &DocumentoAjusteResultado{Company: company, PointOfSale: pointOfSale, Response: &result}, nil
}
