package siat

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
)

type CuisService struct {
	client *Client
	logger *slog.Logger
}

func NewCuisService(client *Client) *CuisService {
	return &CuisService{client: client, logger: nil}
}

type cuisSOAPRequestEnvelope struct {
	XMLName  xml.Name     `xml:"soapenv:Envelope"`
	XmlnsSo  string       `xml:"xmlns:soapenv,attr"`
	XmlnsXsd string       `xml:"xmlns:xsd,attr,omitempty"`
	XmlnsXsi string       `xml:"xmlns:xsi,attr,omitempty"`
	XmlnsNs  string       `xml:"xmlns:ns,attr,omitempty"`
	Body     cuisSOAPBody `xml:"soapenv:Body"`
}

type cuisSOAPBody struct {
	Request cuisOperation `xml:"ns:cuis"`
}

type cuisOperation struct {
	SolicitudCuis SolicitudCuis `xml:"SolicitudCuis"`
}

type cuisSOAPResponseEnvelope struct {
	XMLName xml.Name             `xml:"Envelope"`
	Body    cuisSOAPResponseBody `xml:"Body"`
}

type cuisSOAPResponseBody struct {
	Fault        *SOAPFault             `xml:"Fault"`
	CuisResponse *cuisOperationResponse `xml:"cuisResponse"`
}

type cuisOperationResponse struct {
	RespuestaCuis RespuestaCuis `xml:"RespuestaCuis"`
}

func (s *CuisService) SolicitarCUIS(ctx context.Context, req SolicitudCuis) (*RespuestaCuis, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if s.client == nil {
		return nil, fmt.Errorf("siat cuis service: client is nil")
	}

	requestEnvelope := cuisSOAPRequestEnvelope{
		XmlnsSo:  soapEnvelopeNamespace,
		XmlnsNs:  siatNamespace,
		XmlnsXsd: "http://www.w3.org/2001/XMLSchema",
		XmlnsXsi: "http://www.w3.org/2001/XMLSchema-instance",
		Body: cuisSOAPBody{
			Request: cuisOperation{SolicitudCuis: req},
		},
	}

	payload, err := xml.Marshal(requestEnvelope)
	if err != nil {
		return nil, fmt.Errorf("siat cuis: marshal request: %w", err)
	}
	if err := ValidateXML(payload, "codigoAmbiente", "codigoSistema", "nit", "codigoSucursal", "codigoModalidad", "codigoPuntoVenta"); err != nil {
		return nil, fmt.Errorf("siat cuis: %w", err)
	}

	rawResponse, err := s.client.DoRaw(ctx, "cuis", payload)
	if err != nil {
		return nil, err
	}

	var envelope cuisSOAPResponseEnvelope
	if err := xml.Unmarshal(rawResponse, &envelope); err != nil {
		return nil, fmt.Errorf("siat cuis: unmarshal response: %w", err)
	}
	if envelope.Body.Fault != nil {
		return nil, &SOAPFaultError{Operation: "cuis", Fault: *envelope.Body.Fault}
	}
	if envelope.Body.CuisResponse == nil {
		return nil, fmt.Errorf("siat cuis: response body missing cuisResponse")
	}

	respuesta := envelope.Body.CuisResponse.RespuestaCuis
	if s.logger != nil {
		s.logger.Info("siat cuis parsed", "codigo", respuesta.Codigo, "fecha_vigencia", respuesta.FechaVigencia.String(), "transaccion", respuesta.Transaccion)
	}

	return &respuesta, nil
}
