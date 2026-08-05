package siat

import (
	"context"
	"encoding/xml"
	"fmt"
)

type SolicitudSincronizacion struct {
	XMLName          xml.Name `xml:"SolicitudSincronizacion" json:"-"`
	CodigoAmbiente   int      `xml:"codigoAmbiente" json:"codigoAmbiente"`
	CodigoPuntoVenta *int     `xml:"codigoPuntoVenta,omitempty" json:"codigoPuntoVenta,omitempty"`
	CodigoSistema    string   `xml:"codigoSistema" json:"codigoSistema"`
	CodigoSucursal   int      `xml:"codigoSucursal" json:"codigoSucursal"`
	Cuis             string   `xml:"cuis" json:"cuis"`
	Nit              string   `xml:"nit" json:"nit"`
}

type ParametricaDto struct {
	CodigoClasificador int    `xml:"codigoClasificador" json:"codigoClasificador"`
	Descripcion        string `xml:"descripcion" json:"descripcion"`
}

type RespuestaListaParametricas struct {
	Transaccion  bool             `xml:"transaccion" json:"transaccion"`
	Mensajes     []Mensaje        `xml:"mensajesList>mensaje,omitempty" json:"mensajes,omitempty"`
	ListaCodigos []ParametricaDto `xml:"listaCodigos,omitempty" json:"listaCodigos,omitempty"`
}

type sincronizacionSOAPRequestEnvelope struct {
	XMLName  xml.Name               `xml:"soapenv:Envelope"`
	XmlnsSo  string                 `xml:"xmlns:soapenv,attr"`
	XmlnsXsd string                 `xml:"xmlns:xsd,attr,omitempty"`
	XmlnsXsi string                 `xml:"xmlns:xsi,attr,omitempty"`
	XmlnsNs  string                 `xml:"xmlns:ns,attr,omitempty"`
	Body     sincronizacionSOAPBody `xml:"soapenv:Body"`
}

type sincronizacionSOAPBody struct {
	Request sincronizacionOperation `xml:"ns:sincronizarParametricaTipoPuntoVenta"`
}

type sincronizacionOperation struct {
	SolicitudSincronizacion SolicitudSincronizacion `xml:"SolicitudSincronizacion"`
}

type sincronizacionSOAPResponseEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    struct {
		Fault *SOAPFault `xml:"Fault"`
		Resp  *struct {
			RespuestaListaParametricas RespuestaListaParametricas `xml:"RespuestaListaParametricas"`
		} `xml:"sincronizarParametricaTipoPuntoVentaResponse"`
	} `xml:"Body"`
}

type SincronizacionService struct {
	client *Client
}

func NewSincronizacionService(client *Client) *SincronizacionService {
	return &SincronizacionService{client: client}
}

// SincronizarTipoPuntoVenta baja la parametría oficial de tipos de punto de venta
// (código clasificador + descripción) contra el servicio FacturacionSincronizacion.
func (s *SincronizacionService) SincronizarTipoPuntoVenta(ctx context.Context, req SolicitudSincronizacion) (*RespuestaListaParametricas, error) {
	if s.client == nil {
		return nil, fmt.Errorf("siat sincronizacion service: client is nil")
	}

	endpoint := s.client.ServiceEndpoint(ServiceSincronizacion)
	c := s.client.CloneWithEndpoint(endpoint)

	requestEnvelope := sincronizacionSOAPRequestEnvelope{
		XmlnsSo:  soapEnvelopeNamespace,
		XmlnsNs:  siatNamespace,
		XmlnsXsd: "http://www.w3.org/2001/XMLSchema",
		XmlnsXsi: "http://www.w3.org/2001/XMLSchema-instance",
		Body: sincronizacionSOAPBody{
			Request: sincronizacionOperation{SolicitudSincronizacion: req},
		},
	}

	payload, err := xml.Marshal(requestEnvelope)
	if err != nil {
		return nil, fmt.Errorf("siat sincronizacion: marshal request: %w", err)
	}
	if err := ValidateXML(payload, "codigoAmbiente", "codigoSistema", "codigoSucursal", "cuis", "nit"); err != nil {
		return nil, fmt.Errorf("siat sincronizacion: %w", err)
	}

	rawResponse, err := c.DoRaw(ctx, "sincronizarParametricaTipoPuntoVenta", payload)
	if err != nil {
		return nil, err
	}

	var envelope sincronizacionSOAPResponseEnvelope
	if err := xml.Unmarshal(rawResponse, &envelope); err != nil {
		return nil, fmt.Errorf("siat sincronizacion: unmarshal response: %w", err)
	}
	if envelope.Body.Fault != nil {
		return nil, &SOAPFaultError{Operation: "sincronizarParametricaTipoPuntoVenta", Fault: *envelope.Body.Fault}
	}
	if envelope.Body.Resp == nil {
		return nil, fmt.Errorf("siat sincronizacion: response body missing sincronizarParametricaTipoPuntoVentaResponse")
	}

	respuesta := envelope.Body.Resp.RespuestaListaParametricas
	return &respuesta, nil
}
