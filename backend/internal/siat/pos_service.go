package siat

import (
	"context"
	"encoding/xml"
	"fmt"
)

type RegistroPuntoVentaRequest struct {
	CodigoAmbiente       int    `json:"codigoAmbiente"`
	CodigoModalidad      int    `json:"codigoModalidad"`
	Nit                  string `json:"nit"`
	CodigoSistema        string `json:"codigoSistema"`
	CodigoSucursal       int    `json:"codigoSucursal"`
	NombrePuntoVenta     string `json:"nombrePuntoVenta"`
	CodigoTipoPuntoVenta int    `json:"codigoTipoPuntoVenta"`
	Cuis                 string `json:"cuis"`
	Descripcion          string `json:"descripcion"`
}

type RegistroPuntoVentaResponse struct {
	CodigoPuntoVenta int       `xml:"codigoPuntoVenta" json:"codigoPuntoVenta"`
	Transaccion      bool      `xml:"transaccion" json:"transaccion"`
	Mensajes         []Mensaje `xml:"mensajesList>mensaje,omitempty" json:"mensajes,omitempty"`

	// RawRequest / RawResponse capturan el payload SOAP enviado y recibido para
	// diagnóstico y persistencia.
	RawRequest  string `xml:"-" json:"-"`
	RawResponse string `xml:"-" json:"-"`
}

// ConsultaPuntoVenta consulta el estado de los puntos de venta registrados en el
// SIAT para un NIT/sucursal (operación consultaPuntoVenta).
type ConsultaPuntoVentaRequest struct {
	CodigoAmbiente   int    `json:"codigoAmbiente"`
	CodigoSistema    string `json:"codigoSistema"`
	CodigoSucursal   int    `json:"codigoSucursal"`
	Cuis             string `json:"cuis"`
	Nit              string `json:"nit"`
	CodigoPuntoVenta *int   `json:"codigoPuntoVenta,omitempty"`
}

type PuntoVentaDto struct {
	CodigoTipoPuntoVenta int    `xml:"codigoTipoPuntoVenta" json:"codigoTipoPuntoVenta"`
	NombrePuntoVenta     string `xml:"nombrePuntoVenta" json:"nombrePuntoVenta"`
	Descripcion          string `xml:"descripcion" json:"descripcion"`
	Estado               string `xml:"estado" json:"estado"`
	FechaRegistro        string `xml:"fechaRegistro" json:"fechaRegistro"`
}

type RespuestaConsultaPuntoVenta struct {
	Transaccion      bool            `xml:"transaccion" json:"transaccion"`
	Mensajes         []Mensaje       `xml:"mensajesList>mensaje,omitempty" json:"mensajes,omitempty"`
	ListaPuntosVenta []PuntoVentaDto `xml:"listaPuntosVenta,omitempty" json:"listaPuntosVenta,omitempty"`
	RawRequest       string          `xml:"-" json:"-"`
	RawResponse      string          `xml:"-" json:"-"`
}

// CierrePuntoVenta cierra (anula/deshabilita) un punto de venta ante el SIAT
// (operación cierrePuntoVenta).
type CierrePuntoVentaRequest struct {
	CodigoAmbiente   int    `json:"codigoAmbiente"`
	CodigoSistema    string `json:"codigoSistema"`
	CodigoSucursal   int    `json:"codigoSucursal"`
	Cuis             string `json:"cuis"`
	Nit              string `json:"nit"`
	CodigoPuntoVenta int    `json:"codigoPuntoVenta"`
}

type RespuestaCierrePuntoVenta struct {
	CodigoPuntoVenta int       `xml:"codigoPuntoVenta" json:"codigoPuntoVenta"`
	Transaccion      bool      `xml:"transaccion" json:"transaccion"`
	Mensajes         []Mensaje `xml:"mensajesList>mensaje,omitempty" json:"mensajes,omitempty"`
	RawRequest       string    `xml:"-" json:"-"`
	RawResponse      string    `xml:"-" json:"-"`
}

type PuntoVentaService struct {
	client *Client
}

func NewPuntoVentaService(client *Client) *PuntoVentaService {
	return &PuntoVentaService{client: client}
}

// Registrar envía la operación registroPuntoVenta contra el servicio
// FacturacionOperaciones y devuelve el código oficial asignado por el SIAT.
func (s *PuntoVentaService) Registrar(ctx context.Context, req RegistroPuntoVentaRequest) (*RegistroPuntoVentaResponse, error) {
	if s.client == nil {
		return nil, fmt.Errorf("siat punto venta service: client is nil")
	}

	raw := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="%s" xmlns:ns="%s">`+
		`<soapenv:Body>`+
		`<ns:registroPuntoVenta>`+
		`<SolicitudRegistroPuntoVenta>`+
		`<codigoAmbiente>%d</codigoAmbiente>`+
		`<codigoModalidad>%d</codigoModalidad>`+
		`<codigoSistema>%s</codigoSistema>`+
		`<codigoSucursal>%d</codigoSucursal>`+
		`<codigoTipoPuntoVenta>%d</codigoTipoPuntoVenta>`+
		`<cuis>%s</cuis>`+
		`<descripcion>%s</descripcion>`+
		`<nit>%s</nit>`+
		`<nombrePuntoVenta>%s</nombrePuntoVenta>`+
		`</SolicitudRegistroPuntoVenta>`+
		`</ns:registroPuntoVenta>`+
		`</soapenv:Body>`+
		`</soapenv:Envelope>`, soapEnvelopeNamespace, siatNamespace,
		req.CodigoAmbiente, req.CodigoModalidad, req.CodigoSistema, req.CodigoSucursal,
		req.CodigoTipoPuntoVenta, req.Cuis, req.Descripcion, req.Nit, req.NombrePuntoVenta)

	if err := ValidateXML([]byte(raw), "codigoAmbiente", "codigoModalidad", "codigoSistema",
		"codigoSucursal", "codigoTipoPuntoVenta", "cuis", "nit", "nombrePuntoVenta"); err != nil {
		return nil, fmt.Errorf("siat registroPuntoVenta: %w", err)
	}

	rawResp, err := s.doOperaciones(ctx, "registroPuntoVenta", []byte(raw))
	if err != nil {
		return nil, err
	}

	var respEnvelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Fault    *SOAPFault `xml:"Fault"`
			Response *struct {
				RegistroPuntoVentaResponse RegistroPuntoVentaResponse `xml:"RespuestaRegistroPuntoVenta"`
			} `xml:"registroPuntoVentaResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(rawResp, &respEnvelope); err != nil {
		return nil, fmt.Errorf("siat punto venta: unmarshal response: %w", err)
	}
	if respEnvelope.Body.Fault != nil {
		return nil, &SOAPFaultError{Operation: "registroPuntoVenta", Fault: *respEnvelope.Body.Fault}
	}
	if respEnvelope.Body.Response == nil {
		return nil, fmt.Errorf("siat punto venta: response missing")
	}
	r := respEnvelope.Body.Response.RegistroPuntoVentaResponse
	r.RawRequest = raw
	r.RawResponse = string(rawResp)
	return &r, nil
}

// Consultar envía la operación consultaPuntoVenta y devuelve la lista de puntos
// de venta registrados en el SIAT para la sucursal.
func (s *PuntoVentaService) Consultar(ctx context.Context, req ConsultaPuntoVentaRequest) (*RespuestaConsultaPuntoVenta, error) {
	if s.client == nil {
		return nil, fmt.Errorf("siat punto venta service: client is nil")
	}

	codigoPV := ""
	if req.CodigoPuntoVenta != nil {
		codigoPV = fmt.Sprintf(`<codigoPuntoVenta>%d</codigoPuntoVenta>`, *req.CodigoPuntoVenta)
	}
	raw := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="%s" xmlns:ns="%s">`+
		`<soapenv:Body>`+
		`<ns:consultaPuntoVenta>`+
		`<SolicitudConsultaPuntoVenta>`+
		`<codigoAmbiente>%d</codigoAmbiente>`+
		`<codigoSistema>%s</codigoSistema>`+
		`<codigoSucursal>%d</codigoSucursal>`+
		`<cuis>%s</cuis>`+
		`<nit>%s</nit>`+
		`%s`+
		`</SolicitudConsultaPuntoVenta>`+
		`</ns:consultaPuntoVenta>`+
		`</soapenv:Body>`+
		`</soapenv:Envelope>`, soapEnvelopeNamespace, siatNamespace,
		req.CodigoAmbiente, req.CodigoSistema, req.CodigoSucursal, req.Cuis, req.Nit, codigoPV)

	if err := ValidateXML([]byte(raw), "codigoAmbiente", "codigoSistema", "codigoSucursal", "cuis", "nit"); err != nil {
		return nil, fmt.Errorf("siat consultaPuntoVenta: %w", err)
	}

	rawResp, err := s.doOperaciones(ctx, "consultaPuntoVenta", []byte(raw))
	if err != nil {
		return nil, err
	}

	var respEnvelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Fault    *SOAPFault `xml:"Fault"`
			Response *struct {
				RespuestaConsultaPuntoVenta RespuestaConsultaPuntoVenta `xml:"RespuestaConsultaPuntoVenta"`
			} `xml:"consultaPuntoVentaResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(rawResp, &respEnvelope); err != nil {
		return nil, fmt.Errorf("siat punto venta: unmarshal consulta response: %w", err)
	}
	if respEnvelope.Body.Fault != nil {
		return nil, &SOAPFaultError{Operation: "consultaPuntoVenta", Fault: *respEnvelope.Body.Fault}
	}
	if respEnvelope.Body.Response == nil {
		return nil, fmt.Errorf("siat punto venta: consulta response missing")
	}
	r := respEnvelope.Body.Response.RespuestaConsultaPuntoVenta
	r.RawRequest = raw
	r.RawResponse = string(rawResp)
	return &r, nil
}

// Cerrar envía la operación cierrePuntoVenta para deshabilitar el punto de venta
// ante el SIAT.
func (s *PuntoVentaService) Cerrar(ctx context.Context, req CierrePuntoVentaRequest) (*RespuestaCierrePuntoVenta, error) {
	if s.client == nil {
		return nil, fmt.Errorf("siat punto venta service: client is nil")
	}

	raw := fmt.Sprintf(`<soapenv:Envelope xmlns:soapenv="%s" xmlns:ns="%s">`+
		`<soapenv:Body>`+
		`<ns:cierrePuntoVenta>`+
		`<SolicitudCierrePuntoVenta>`+
		`<codigoAmbiente>%d</codigoAmbiente>`+
		`<codigoSistema>%s</codigoSistema>`+
		`<codigoSucursal>%d</codigoSucursal>`+
		`<cuis>%s</cuis>`+
		`<nit>%s</nit>`+
		`<codigoPuntoVenta>%d</codigoPuntoVenta>`+
		`</SolicitudCierrePuntoVenta>`+
		`</ns:cierrePuntoVenta>`+
		`</soapenv:Body>`+
		`</soapenv:Envelope>`, soapEnvelopeNamespace, siatNamespace,
		req.CodigoAmbiente, req.CodigoSistema, req.CodigoSucursal, req.Cuis, req.Nit, req.CodigoPuntoVenta)

	if err := ValidateXML([]byte(raw), "codigoAmbiente", "codigoSistema", "codigoSucursal",
		"cuis", "nit", "codigoPuntoVenta"); err != nil {
		return nil, fmt.Errorf("siat cierrePuntoVenta: %w", err)
	}

	rawResp, err := s.doOperaciones(ctx, "cierrePuntoVenta", []byte(raw))
	if err != nil {
		return nil, err
	}

	var respEnvelope struct {
		XMLName xml.Name `xml:"Envelope"`
		Body    struct {
			Fault    *SOAPFault `xml:"Fault"`
			Response *struct {
				RespuestaCierrePuntoVenta RespuestaCierrePuntoVenta `xml:"RespuestaCierrePuntoVenta"`
			} `xml:"cierrePuntoVentaResponse"`
		} `xml:"Body"`
	}
	if err := xml.Unmarshal(rawResp, &respEnvelope); err != nil {
		return nil, fmt.Errorf("siat punto venta: unmarshal cierre response: %w", err)
	}
	if respEnvelope.Body.Fault != nil {
		return nil, &SOAPFaultError{Operation: "cierrePuntoVenta", Fault: *respEnvelope.Body.Fault}
	}
	if respEnvelope.Body.Response == nil {
		return nil, fmt.Errorf("siat punto venta: cierre response missing")
	}
	r := respEnvelope.Body.Response.RespuestaCierrePuntoVenta
	r.RawRequest = raw
	r.RawResponse = string(rawResp)
	return &r, nil
}

// doOperaciones envía el payload SOAP crudo contra el servicio
// FacturacionOperaciones (registro/consulta/cierre de puntos de venta).
func (s *PuntoVentaService) doOperaciones(ctx context.Context, operation string, rawPayload []byte) ([]byte, error) {
	opClient := s.client.CloneWithEndpoint(s.client.ServiceEndpoint(ServiceOperaciones))
	rawResp, err := opClient.DoRaw(ctx, operation, rawPayload)
	if err != nil {
		return nil, err
	}
	if err := ValidateXML(rawResp); err != nil {
		return nil, fmt.Errorf("siat %s: respuesta inválida: %w", operation, err)
	}
	return rawResp, nil
}
