package siat

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// ValidateXML valida estrictamente un payload SOAP antes de enviarlo o procesarlo:
//
//   - XML bien formado (encoding/xml.Decoder)
//   - UTF-8 válido
//   - sin caracteres de control ilegales (excepto \t \n \r)
//   - presencia del sobre SOAP: elemento Envelope con su Body
//   - etiquetas obligatorias (pasadas en required) presentes y no vacías
//
// El SIAT rechaza requests con XML mal formado o namespaces SOAP ausentes.
func ValidateXML(data []byte, required ...string) error {
	if len(bytes.TrimSpace(data)) == 0 {
		return fmt.Errorf("siat: XML vacío")
	}
	if !utf8.Valid(data) {
		return fmt.Errorf("siat: XML no es UTF-8 válido")
	}
	if hasIllegalControlChars(data) {
		return fmt.Errorf("siat: XML contiene caracteres de control no permitidos")
	}

	dec := xml.NewDecoder(bytes.NewReader(data))

	requiredSet := make(map[string]struct{}, len(required))
	for _, tag := range required {
		requiredSet[tag] = struct{}{}
	}

	type stackEntry struct {
		name       string
		sawContent bool
	}
	var stack []stackEntry
	foundEnvelope := false
	foundBody := false
	requiredSeen := make(map[string]struct{}, len(required))

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("siat: XML mal formado: %w", err)
		}
		switch t := tok.(type) {
		case xml.StartElement:
			foundEnvelope = foundEnvelope || t.Name.Local == "Envelope"
			foundBody = foundBody || t.Name.Local == "Body"
			stack = append(stack, stackEntry{name: t.Name.Local})
			if _, ok := requiredSet[t.Name.Local]; ok {
				requiredSeen[t.Name.Local] = struct{}{}
			}
		case xml.EndElement:
			if len(stack) == 0 {
				return fmt.Errorf("siat: XML mal formado: cierre inesperado </%s>", t.Name.Local)
			}
			top := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if _, ok := requiredSet[top.name]; ok && !top.sawContent {
				return fmt.Errorf("siat: la etiqueta obligatoria <%s> está vacía", top.name)
			}
		case xml.CharData:
			if len(stack) > 0 && len(strings.TrimSpace(string(t))) > 0 {
				stack[len(stack)-1].sawContent = true
			}
		}
	}

	if len(stack) > 0 {
		return fmt.Errorf("siat: XML mal formado: faltan etiquetas de cierre")
	}
	if !foundEnvelope || !foundBody {
		return fmt.Errorf("siat: el sobre SOAP no incluye Envelope/Body")
	}
	for _, tag := range required {
		if _, ok := requiredSeen[tag]; !ok {
			return fmt.Errorf("siat: falta la etiqueta obligatoria <%s>", tag)
		}
	}
	return nil
}

func hasIllegalControlChars(data []byte) bool {
	for _, b := range data {
		if b < 0x20 && b != '\t' && b != '\n' && b != '\r' {
			return true
		}
	}
	return false
}
