package siat

import (
	"context"
	"encoding/xml"
	"fmt"
	"log/slog"
)

type CufdService struct {
	client *Client
	logger *slog.Logger
}

func NewCufdService(client *Client) *CufdService {
	return &CufdService{client: client, logger: nil}
}

type cufdSOAPRequestEnvelope struct {
	XMLName  xml.Name     `xml:"soapenv:Envelope"`
	XmlnsSo  string       `xml:"xmlns:soapenv,attr"`
	XmlnsXsd string       `xml:"xmlns:xsd,attr,omitempty"`
	XmlnsXsi string       `xml:"xmlns:xsi,attr,omitempty"`
	XmlnsNs  string       `xml:"xmlns:ns,attr,omitempty"`
	Body     cufdSOAPBody `xml:"soapenv:Body"`
}

type cufdSOAPBody struct {
	Request cufdOperation `xml:"ns:cufd"`
}

type cufdOperation struct {
	SolicitudCufd SolicitudCufd `xml:"SolicitudCufd"`
}

type cufdSOAPResponseEnvelope struct {
	XMLName xml.Name             `xml:"Envelope"`
	Body    cufdSOAPResponseBody `xml:"Body"`
}

type cufdSOAPResponseBody struct {
	Fault        *SOAPFault             `xml:"Fault"`
	CufdResponse *cufdOperationResponse `xml:"cufdResponse"`
}

type cufdOperationResponse struct {
	RespuestaCufd RespuestaCufd `xml:"RespuestaCufd"`
}

func (s *CufdService) SolicitarCUFD(ctx context.Context, req SolicitudCufd) (*RespuestaCufd, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	if s.client == nil {
		return nil, fmt.Errorf("siat cufd service: client is nil")
	}

	requestEnvelope := cufdSOAPRequestEnvelope{
		XmlnsSo:  soapEnvelopeNamespace,
		XmlnsNs:  siatNamespace,
		XmlnsXsd: "http://www.w3.org/2001/XMLSchema",
		XmlnsXsi: "http://www.w3.org/2001/XMLSchema-instance",
		Body: cufdSOAPBody{
			Request: cufdOperation{SolicitudCufd: req},
		},
	}

	payload, err := xml.Marshal(requestEnvelope)
	if err != nil {
		return nil, fmt.Errorf("siat cufd: marshal request: %w", err)
	}
	if err := ValidateXML(payload, "codigoAmbiente", "codigoSistema", "nit", "codigoSucursal", "cuis", "codigoModalidad", "codigoPuntoVenta"); err != nil {
		return nil, fmt.Errorf("siat cufd: %w", err)
	}

	rawResponse, err := s.client.DoRaw(ctx, "cufd", payload)
	if err != nil {
		return nil, err
	}

	var envelope cufdSOAPResponseEnvelope
	if err := xml.Unmarshal(rawResponse, &envelope); err != nil {
		return nil, fmt.Errorf("siat cufd: unmarshal response: %w", err)
	}
	if envelope.Body.Fault != nil {
		return nil, &SOAPFaultError{Operation: "cufd", Fault: *envelope.Body.Fault}
	}
	if envelope.Body.CufdResponse == nil {
		return nil, fmt.Errorf("siat cufd: response body missing cufdResponse")
	}

	respuesta := envelope.Body.CufdResponse.RespuestaCufd
	if s.logger != nil {
		s.logger.Info("siat cufd parsed", "codigo", respuesta.Codigo, "codigo_control", respuesta.CodigoControl, "fecha_vigencia", respuesta.FechaVigencia.String(), "transaccion", respuesta.Transaccion)
	}

	return &respuesta, nil
}
