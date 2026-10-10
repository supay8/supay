package siat

import (
	"context"
	"encoding/xml"

	"github.com/brandsrx/supay/internal/adapters/siat/batch"
	"github.com/brandsrx/supay/internal/adapters/siat/single"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

var _ ports.DocumentSerializer = (*FiscalAdapter)(nil)
var _ ports.FiscalEmissionPipeline = (*FiscalAdapter)(nil)

func (a *FiscalAdapter) Serialize(_ context.Context, doc ports.FiscalDocument, emission int) (ports.SerializedDocument, error) {
	req := toSiatSolicitudFactura(doc)
	if err := a.svc.applyIdentity(&req); err != nil {
		return ports.SerializedDocument{}, domain.NewBadRequestError(err.Error())
	}
	if err := req.validate(); err != nil {
		return ports.SerializedDocument{}, domain.NewBadRequestError(err.Error())
	}
	invoice, cuf, kind, err := buildFacturaSDK(req, emission)
	if err != nil {
		return ports.SerializedDocument{}, domain.NewBadRequestError(err.Error())
	}
	data, err := xml.Marshal(invoice)
	if err != nil {
		return ports.SerializedDocument{}, domain.NewBadRequestError(err.Error())
	}
	return ports.SerializedDocument{XML: removeEmptyOptionalFacturaFields(data), CUF: cuf, DocumentType: kind}, nil
}

func (a *FiscalAdapter) PrepareEmission(ctx context.Context, doc ports.FiscalDocument) (ports.FiscalResult, error) {
	return (single.Preparer{Serializer: a, Signer: a, Packer: batch.Packer{}}).Prepare(ctx, doc, fiscal.EmisionOnline)
}

func (a *FiscalAdapter) DispatchEmission(ctx context.Context, doc ports.FiscalDocument, prepared ports.FiscalResult) (ports.FiscalResult, error) {
	doc.XML, doc.Cuf, doc.Archivo, doc.HashArchivo = prepared.Xml, prepared.Cuf, prepared.Archivo, prepared.XmlHash
	return a.Emit(ctx, doc)
}
