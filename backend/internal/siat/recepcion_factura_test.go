package siat

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"io"
	"testing"

	"github.com/brandsrx/supay/internal/models"
)

func TestCompressAndHash(t *testing.T) {
	input := []byte("<?xml version=\"1.0\"?><factura>prueba</factura>")

	hashHex, archivo, err := CompressAndHash(input)
	if err != nil {
		t.Fatalf("CompressAndHash: %v", err)
	}

	// El archivo debe ser base64 de un gzip válido que descomprime al original.
	compressed, err := base64.StdEncoding.DecodeString(archivo)
	if err != nil {
		t.Fatalf("archivo no es base64 válido: %v", err)
	}
	gz, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("archivo no es gzip válido: %v", err)
	}
	roundtrip, err := io.ReadAll(gz)
	if err != nil {
		t.Fatalf("decompress: %v", err)
	}
	if !bytes.Equal(roundtrip, input) {
		t.Fatalf("roundtrip difiere\n got: %q\nwant: %q", roundtrip, input)
	}

	// El hash debe ser SHA256 hexadecimal del archivo comprimido.
	sum := sha256.Sum256(compressed)
	if hashHex != hex.EncodeToString(sum[:]) {
		t.Fatalf("hash incorrecto\n got: %s\nwant: %s", hashHex, hex.EncodeToString(sum[:]))
	}
}

func TestParseRespuestaServicioFacturacion(t *testing.T) {
	raw := `<?xml version="1.0" encoding="UTF-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <ns:recepcionFacturaResponse xmlns:ns="https://siat.impuestos.gob.bo">
      <RespuestaServicioFacturacion>
        <codigoDescripcion>Correcta</codigoDescripcion>
        <codigoEstado>908</codigoEstado>
        <codigoRecepcion>199301</codigoRecepcion>
        <transaccion>true</transaccion>
        <mensajesList>
          <codigo>979</codigo>
          <descripcion>FACTURA PROCESADA CORRECTAMENTE</descripcion>
          <advertencia>false</advertencia>
          <numeroArchivo>1</numeroArchivo>
          <numeroDetalle>0</numeroDetalle>
        </mensajesList>
      </RespuestaServicioFacturacion>
    </ns:recepcionFacturaResponse>
  </soapenv:Body>
</soapenv:Envelope>`

	resp, err := parseRespuestaServicioFacturacion([]byte(raw))
	if err != nil {
		t.Fatalf("parseRespuestaServicioFacturacion: %v", err)
	}
	if resp.CodigoEstado != "908" {
		t.Errorf("codigoEstado = %q; se esperaba 908", resp.CodigoEstado)
	}
	if resp.CodigoRecepcion != "199301" {
		t.Errorf("codigoRecepcion = %q; se esperaba 199301", resp.CodigoRecepcion)
	}
	if !resp.Transaccion {
		t.Errorf("transaccion = false; se esperaba true")
	}
	if len(resp.Mensajes) != 1 {
		t.Fatalf("mensajes len = %d; se esperaba 1", len(resp.Mensajes))
	}
	m := resp.Mensajes[0]
	if m.Codigo != 979 || m.Descripcion != "FACTURA PROCESADA CORRECTAMENTE" || m.NumeroArchivo != 1 {
		t.Errorf("mensaje parseado incorrecto: %+v", m)
	}
}

func TestParseRespuestaServicioFacturacion_Fault(t *testing.T) {
	raw := `<?xml version="1.0"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body>
    <soapenv:Fault>
      <faultcode>soapenv:Server</faultcode>
      <faultstring>Error interno del servicio</faultstring>
    </soapenv:Fault>
  </soapenv:Body>
</soapenv:Envelope>`

	_, err := parseRespuestaServicioFacturacion([]byte(raw))
	if err == nil {
		t.Fatal("se esperaba error por SOAP Fault")
	}
	var faultErr *SOAPFaultError
	if !errors.As(err, &faultErr) {
		t.Fatalf("error no es SOAPFaultError: %v", err)
	}
	if faultErr.Fault.Code != "soapenv:Server" || faultErr.Fault.String != "Error interno del servicio" {
		t.Errorf("fault parseado incorrecto: %+v", faultErr.Fault)
	}
}

func TestParseRespuestaServicioFacturacion_SinRespuesta(t *testing.T) {
	raw := `<?xml version="1.0"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
  <soapenv:Body><otraOperacion/></soapenv:Body>
</soapenv:Envelope>`

	if _, err := parseRespuestaServicioFacturacion([]byte(raw)); err == nil {
		t.Fatal("se esperaba error por cuerpo sin recepcionFacturaResponse")
	}
}

func TestParseRespuestaServicioFacturacion_XMLInvalido(t *testing.T) {
	if _, err := parseRespuestaServicioFacturacion([]byte("<Envelope><Body>")); err == nil {
		t.Fatal("se esperaba error por XML inválido")
	}
}

func TestEmissionStatus(t *testing.T) {
	tests := []struct {
		codigo string
		want   models.InvoiceStatus
	}{
		{"908", models.StatusAccepted},
		{"903", models.StatusAccepted},
		{"902", models.StatusRejected},
		{"904", models.StatusObserved},
		{"901", models.StatusPending},
		{"999", models.StatusSent},
		{"", models.StatusSent},
	}
	for _, tt := range tests {
		if got := emissionStatus(tt.codigo); got != tt.want {
			t.Errorf("emissionStatus(%q) = %q; se esperaba %q", tt.codigo, got, tt.want)
		}
	}
}
