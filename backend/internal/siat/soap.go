package siat

import (
	"bytes"
	"encoding/xml"
	"fmt"
)

const (
	soapEnvelopeNamespace = "http://schemas.xmlsoap.org/soap/envelope/"
	soapContentType       = "text/xml; charset=utf-8"
)

type SOAPEnvelope struct {
	XMLName xml.Name `xml:"Envelope"`
	Body    SOAPBody `xml:"Body"`
}

type SOAPBody struct {
	Fault *SOAPFault `xml:"Fault"`
}

type SOAPFault struct {
	Code   string `xml:"faultcode" json:"faultcode"`
	String string `xml:"faultstring" json:"faultstring"`
	Detail string `xml:"detail" json:"detail"`
}

type SOAPFaultError struct {
	Operation string
	Fault     SOAPFault
}

func (e *SOAPFaultError) Error() string {
	if e == nil {
		return "siat soap fault"
	}
	if e.Operation != "" {
		return fmt.Sprintf("siat soap fault in %s: %s - %s", e.Operation, e.Fault.Code, e.Fault.String)
	}
	return fmt.Sprintf("siat soap fault: %s - %s", e.Fault.Code, e.Fault.String)
}

func parseSOAPFault(payload []byte) (*SOAPFault, error) {
	if len(bytes.TrimSpace(payload)) == 0 {
		return nil, nil
	}

	var envelope SOAPEnvelope
	if err := xml.Unmarshal(payload, &envelope); err != nil {
		return nil, err
	}

	if envelope.Body.Fault == nil {
		return nil, nil
	}

	return envelope.Body.Fault, nil
}
