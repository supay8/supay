package siat

import (
	"strings"
	"testing"
)

func TestValidateXML_ValidEnvelope(t *testing.T) {
	data := []byte(`<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
		<soapenv:Body><ns:op><codigoAmbiente>2</codigoAmbiente><nit>9971522011</nit></ns:op></soapenv:Body>
	</soapenv:Envelope>`)
	if err := ValidateXML(data, "codigoAmbiente", "nit"); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateXML_Empty(t *testing.T) {
	if err := ValidateXML(nil, "codigoAmbiente"); err == nil {
		t.Fatal("expected error for empty XML")
	}
}

func TestValidateXML_NotUTF8(t *testing.T) {
	data := []byte{0xff, 0xfe, 0x3c, 0x3e}
	if err := ValidateXML(data); err == nil {
		t.Fatal("expected error for non-UTF8")
	}
}

func TestValidateXML_IllegalControlChar(t *testing.T) {
	data := []byte("<Envelope><Body><x>a\x01b</x></Body></Envelope>")
	if err := ValidateXML(data); err == nil {
		t.Fatal("expected error for illegal control char")
	}
}

func TestValidateXML_Malformed(t *testing.T) {
	data := []byte(`<Envelope><Body><x>unclosed</Body>`)
	err := ValidateXML(data)
	if err == nil {
		t.Fatal("expected error for malformed XML")
	}
	if !strings.Contains(err.Error(), "falta") && !strings.Contains(err.Error(), "mal formado") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateXML_MissingEnvelope(t *testing.T) {
	data := []byte(`<soapenv:Body><x>1</x></soapenv:Body>`)
	if err := ValidateXML(data); err == nil {
		t.Fatal("expected error for missing Envelope")
	}
}

func TestValidateXML_RequiredTagMissing(t *testing.T) {
	data := []byte(`<Envelope><Body><x><codigoAmbiente>2</codigoAmbiente></x></Body></Envelope>`)
	if err := ValidateXML(data, "codigoAmbiente", "nit"); err == nil {
		t.Fatal("expected error for missing required tag")
	}
}

func TestValidateXML_RequiredTagEmpty(t *testing.T) {
	data := []byte(`<Envelope><Body><x><codigoAmbiente></codigoAmbiente></x></Body></Envelope>`)
	if err := ValidateXML(data, "codigoAmbiente"); err == nil {
		t.Fatal("expected error for empty required tag")
	}
}
