package invoiceemail

import (
	"bytes"
	"testing"
)

func TestBuildMIMEMessageIncludesPDFAndXML(t *testing.T) {
	payload, err := buildMIMEMessage(SMTPConfig{From: "billing@example.com", FromName: "Supay"}, Message{
		To:      "customer@example.com",
		Subject: "Factura 42",
		HTML:    "<p>Adjuntos</p>",
		Attachments: []Attachment{
			{Filename: "factura-42.pdf", ContentType: "application/pdf", Data: []byte("pdf-data")},
			{Filename: "factura-42.xml", ContentType: "application/xml", Data: []byte("xml-data")},
		},
	})
	if err != nil {
		t.Fatalf("buildMIMEMessage: %v", err)
	}
	for _, expected := range [][]byte{
		[]byte("Content-Type: multipart/mixed"),
		[]byte(`filename="factura-42.pdf"`),
		[]byte("Content-Type: application/pdf"),
		[]byte(`filename="factura-42.xml"`),
		[]byte("Content-Type: application/xml"),
	} {
		if !bytes.Contains(payload, expected) {
			t.Fatalf("mensaje MIME no contiene %q", expected)
		}
	}
}

func TestBuildMIMEMessageStripsHeaderInjection(t *testing.T) {
	payload, err := buildMIMEMessage(SMTPConfig{From: "billing@example.com"}, Message{
		To:      "customer@example.com\r\nBcc: attacker@example.com",
		Subject: "Factura\r\nX-Evil: yes",
		Text:    "ok",
	})
	if err != nil {
		t.Fatalf("buildMIMEMessage: %v", err)
	}
	if bytes.Contains(payload, []byte("\r\nBcc:")) || bytes.Contains(payload, []byte("\r\nX-Evil:")) {
		t.Fatalf("se permitió inyección de cabeceras:\n%s", payload)
	}
}
