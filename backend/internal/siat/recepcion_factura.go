package siat

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"fmt"
)

// Estados de recepción devueltos por el SIAT en RespuestaServicioFacturacion.
const (
	CodigoEstadoPendiente  = "901" // en proceso de validación
	CodigoEstadoRechazada  = "902" // rechazada
	CodigoEstadoProcesada  = "903" // procesada
	CodigoEstadoObservada  = "904" // observada (validada con observaciones)
	CodigoEstadoValidada   = "908" // validada / aceptada
)

// CompressAndHash comprime el XML firmado en GZIP, calcula el SHA256 del
// archivo comprimido (hashArchivo) y lo codifica en base64 (archivo), tal como
// exige el SIAT para la operación recepcionFactura.
func CompressAndHash(xmlData []byte) (hashHex string, archivoBase64 string, err error) {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(xmlData); err != nil {
		return "", "", fmt.Errorf("siat emit: gzip: %w", err)
	}
	if err := gz.Close(); err != nil {
		return "", "", fmt.Errorf("siat emit: gzip close: %w", err)
	}
	compressed := buf.Bytes()

	sum := sha256.Sum256(compressed)
	return hex.EncodeToString(sum[:]), base64.StdEncoding.EncodeToString(compressed), nil
}

// SolicitudServicioRecepcionFactura es el cuerpo de la operación
// recepcionFactura del servicio ServicioFacturacionElectronica (WSDL v2).
// El orden de los campos debe respetar el xs:sequence del WSDL.
type SolicitudServicioRecepcionFactura struct {
	XMLName               xml.Name `xml:"SolicitudServicioRecepcionFactura"`
	CodigoAmbiente        int      `xml:"codigoAmbiente"`
	CodigoDocumentoSector int      `xml:"codigoDocumentoSector"`
	CodigoEmision         int      `xml:"codigoEmision"`
	CodigoModalidad       int      `xml:"codigoModalidad"`
	CodigoPuntoVenta      int      `xml:"codigoPuntoVenta"`
	CodigoSistema         string   `xml:"codigoSistema"`
	CodigoSucursal        int      `xml:"codigoSucursal"`
	Cufd                  string   `xml:"cufd"`
	Cuis                  string   `xml:"cuis"`
	Nit                   string   `xml:"nit"`
	TipoFacturaDocumento  int      `xml:"tipoFacturaDocumento"`
	Archivo               string   `xml:"archivo"`
	FechaEnvio            string   `xml:"fechaEnvio"`
	HashArchivo           string   `xml:"hashArchivo"`
}

// RespuestaServicioFacturacion es la respuesta de la operación
// recepcionFactura.
type RespuestaServicioFacturacion struct {
	XMLName           xml.Name            `xml:"RespuestaServicioFacturacion"`
	CodigoDescripcion string              `xml:"codigoDescripcion"`
	CodigoEstado      string              `xml:"codigoEstado"`
	CodigoRecepcion   string              `xml:"codigoRecepcion"`
	Transaccion       bool                `xml:"transaccion"`
	Mensajes          []MensajeRecepcion  `xml:"mensajesList"`
}

type MensajeRecepcion struct {
	Codigo        int    `xml:"codigo"`
	Descripcion   string `xml:"descripcion"`
	Advertencia   bool   `xml:"advertencia"`
	NumeroArchivo int    `xml:"numeroArchivo"`
	NumeroDetalle int    `xml:"numeroDetalle"`
}

type recepcionFacturaSOAPRequestEnvelope struct {
	XMLName  xml.Name                  `xml:"soapenv:Envelope"`
	XmlnsSo  string                    `xml:"xmlns:soapenv,attr"`
	XmlnsNs  string                    `xml:"xmlns:ns,attr,omitempty"`
	XmlnsXsd string                    `xml:"xmlns:xsd,attr,omitempty"`
	XmlnsXsi string                    `xml:"xmlns:xsi,attr,omitempty"`
	Body     recepcionFacturaSOAPBody  `xml:"soapenv:Body"`
}

type recepcionFacturaSOAPBody struct {
	Request recepcionFacturaOperation `xml:"ns:recepcionFactura"`
}

type recepcionFacturaOperation struct {
	Solicitud SolicitudServicioRecepcionFactura `xml:"SolicitudServicioRecepcionFactura"`
}

type recepcionFacturaSOAPResponseEnvelope struct {
	XMLName xml.Name                         `xml:"Envelope"`
	Body    recepcionFacturaSOAPResponseBody `xml:"Body"`
}

type recepcionFacturaSOAPResponseBody struct {
	Fault    *SOAPFault                         `xml:"Fault"`
	Response *recepcionFacturaOperationResponse `xml:"recepcionFacturaResponse"`
}

type recepcionFacturaOperationResponse struct {
	Respuesta RespuestaServicioFacturacion `xml:"RespuestaServicioFacturacion"`
}

// parseRespuestaServicioFacturacion extrae la respuesta del envelope SOAP.
func parseRespuestaServicioFacturacion(raw []byte) (*RespuestaServicioFacturacion, error) {
	var envelope recepcionFacturaSOAPResponseEnvelope
	if err := xml.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("siat recepcion factura: unmarshal response: %w", err)
	}
	if envelope.Body.Fault != nil {
		return nil, &SOAPFaultError{Operation: "recepcionFactura", Fault: *envelope.Body.Fault}
	}
	if envelope.Body.Response == nil {
		return nil, fmt.Errorf("siat recepcion factura: response body missing recepcionFacturaResponse")
	}
	return &envelope.Body.Response.Respuesta, nil
}
