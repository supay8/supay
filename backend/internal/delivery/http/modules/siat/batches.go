package siat

import (
	"encoding/json"
	"errors"

	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/usecase"
)

type batchSubmissionRequest struct {
	InvoiceIDs invoiceSelection `json:"invoice_ids"`
}

type invoiceSelection []string

// Sólo la ausencia de invoice_ids permite la selección automática. Un null o
// una segunda aparición del campo no deben borrar una selección explícita.
func (ids *invoiceSelection) UnmarshalJSON(data []byte) error {
	if *ids != nil {
		return errors.New("invoice_ids no puede repetirse")
	}
	var selected []*string
	if err := json.Unmarshal(data, &selected); err != nil {
		return err
	}
	if selected == nil {
		return errors.New("invoice_ids debe ser un arreglo")
	}
	values := make(invoiceSelection, len(selected))
	for i, id := range selected {
		if id == nil {
			return errors.New("invoice_ids debe contener únicamente strings")
		}
		values[i] = *id
	}
	*ids = values
	return nil
}

type batchCompanyResponse struct {
	ID           string `json:"id"`
	Nit          string `json:"nit"`
	BusinessName string `json:"business_name"`
}

type batchPointOfSaleResponse struct {
	ID               string `json:"id"`
	CodigoSucursal   int    `json:"codigo_sucursal"`
	CodigoPuntoVenta int    `json:"codigo_punto_venta"`
	Description      string `json:"description"`
}

type batchResponse struct {
	Company     *batchCompanyResponse      `json:"company"`
	PointOfSale *batchPointOfSaleResponse  `json:"point_of_sale"`
	Response    *ports.FiscalPackageResult `json:"response"`
	Batches     []usecase.BatchResultado   `json:"batches"`
}

// La respuesta identifica emisor y punto de venta sin exponer configuración de
// empresa, CUIS ni respuestas internas del registro del punto de venta.
func newBatchResponse(result *usecase.PaqueteResultado) batchResponse {
	response := batchResponse{Response: result.Response, Batches: result.Batches}
	if company := result.Company; company != nil {
		response.Company = &batchCompanyResponse{ID: company.ID, Nit: company.Nit, BusinessName: company.BusinessName}
	}
	if pos := result.PointOfSale; pos != nil {
		response.PointOfSale = &batchPointOfSaleResponse{
			ID: pos.ID, CodigoSucursal: pos.CodigoSucursal,
			CodigoPuntoVenta: pos.CodigoPuntoVenta, Description: pos.Description,
		}
	}
	return response
}
