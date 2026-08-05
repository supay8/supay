package siat

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

type XMLDateTime struct {
	time.Time
}

func (d *XMLDateTime) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if raw == "" {
		d.Time = time.Time{}
		return nil
	}

	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02/01/2006 15:04:05",
	} {
		if parsed, err := time.Parse(layout, raw); err == nil {
			d.Time = parsed
			return nil
		}
	}

	return fmt.Errorf("siat: unsupported date format %q", raw)
}

func (d XMLDateTime) MarshalText() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte{}, nil
	}
	return []byte(d.Time.Format(time.RFC3339Nano)), nil
}

func (d XMLDateTime) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Time.Format(time.RFC3339Nano))
}

func (d *XMLDateTime) UnmarshalXML(decoder *xml.Decoder, start xml.StartElement) error {
	var value string
	if err := decoder.DecodeElement(&value, &start); err != nil {
		return err
	}
	return d.UnmarshalText([]byte(value))
}

func (d *XMLDateTime) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if bytes.Equal(trimmed, []byte("null")) || len(trimmed) == 0 {
		d.Time = time.Time{}
		return nil
	}

	var value string
	if err := json.Unmarshal(trimmed, &value); err != nil {
		return err
	}
	return d.UnmarshalText([]byte(value))
}

func (d XMLDateTime) String() string {
	if d.Time.IsZero() {
		return ""
	}
	return d.Time.Format(time.RFC3339Nano)
}

type Mensaje struct {
	Codigo      string `xml:"codigo" json:"codigo"`
	Descripcion string `xml:"descripcion" json:"descripcion"`
}

type SolicitudCuis struct {
	XMLName xml.Name `xml:"SolicitudCuis" json:"-"`

	CodigoAmbiente   int     `xml:"codigoAmbiente" json:"codigoAmbiente"`
	CodigoSistema    string  `xml:"codigoSistema" json:"codigoSistema"`
	Nit              string  `xml:"nit" json:"nit"`
	CodigoSucursal   int     `xml:"codigoSucursal" json:"codigoSucursal"`
	Cuis             *string `xml:"cuis,omitempty" json:"cuis,omitempty"`
	CodigoModalidad  int     `xml:"codigoModalidad" json:"codigoModalidad"`
	CodigoPuntoVenta int     `xml:"codigoPuntoVenta" json:"codigoPuntoVenta"`
}

type RespuestaCuis struct {
	XMLName        xml.Name    `xml:"RespuestaCuis" json:"-"`
	Codigo         string      `xml:"codigo" json:"codigo"`
	FechaVigencia  XMLDateTime `xml:"fechaVigencia" json:"fechaVigencia"`
	Transaccion    bool        `xml:"transaccion" json:"transaccion"`
	Mensajes       []Mensaje   `xml:"mensajesList,omitempty" json:"mensajes,omitempty"`
	CodigoEstado   string      `xml:"codigoEstado,omitempty" json:"codigoEstado,omitempty"`
	CodigoSistema  string      `xml:"codigoSistema,omitempty" json:"codigoSistema,omitempty"`
	CodigoAmbiente string      `xml:"codigoAmbiente,omitempty" json:"codigoAmbiente,omitempty"`
}

type SolicitudCufd struct {
	XMLName xml.Name `xml:"SolicitudCufd" json:"-"`

	CodigoAmbiente   int    `xml:"codigoAmbiente" json:"codigoAmbiente"`
	CodigoSistema    string `xml:"codigoSistema" json:"codigoSistema"`
	Nit              string `xml:"nit" json:"nit"`
	CodigoSucursal   int    `xml:"codigoSucursal" json:"codigoSucursal"`
	Cuis             string `xml:"cuis" json:"cuis"`
	CodigoModalidad  int    `xml:"codigoModalidad" json:"codigoModalidad"`
	CodigoPuntoVenta int    `xml:"codigoPuntoVenta" json:"codigoPuntoVenta"`
}

type RespuestaCufd struct {
	XMLName       xml.Name    `xml:"RespuestaCufd" json:"-"`
	Codigo        string      `xml:"codigo" json:"codigo"`
	CodigoControl string      `xml:"codigoControl" json:"codigoControl"`
	Direccion     string      `xml:"direccion" json:"direccion"`
	FechaVigencia XMLDateTime `xml:"fechaVigencia" json:"fechaVigencia"`
	Transaccion   bool        `xml:"transaccion" json:"transaccion"`
	Mensajes      []Mensaje   `xml:"mensajesList,omitempty" json:"mensajes,omitempty"`
}

func (s SolicitudCuis) Validate() error {
	if s.CodigoAmbiente <= 0 {
		return fmt.Errorf("siat solicitud cuis: codigoAmbiente is required")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat solicitud cuis: codigoSistema is required")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat solicitud cuis: nit is required")
	}
	if s.CodigoSucursal < 0 {
		return fmt.Errorf("siat solicitud cuis: codigoSucursal must be >= 0")
	}
	if s.CodigoModalidad <= 0 {
		return fmt.Errorf("siat solicitud cuis: codigoModalidad is required")
	}
	if s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat solicitud cuis: codigoPuntoVenta must be >= 0")
	}
	if s.Cuis != nil && strings.TrimSpace(*s.Cuis) == "" {
		return fmt.Errorf("siat solicitud cuis: cuis cannot be empty when provided")
	}
	return nil
}

func (s SolicitudCufd) Validate() error {
	if s.CodigoAmbiente <= 0 {
		return fmt.Errorf("siat solicitud cufd: codigoAmbiente is required")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat solicitud cufd: codigoSistema is required")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat solicitud cufd: nit is required")
	}
	if s.CodigoSucursal < 0 {
		return fmt.Errorf("siat solicitud cufd: codigoSucursal must be >= 0")
	}
	if strings.TrimSpace(s.Cuis) == "" {
		return fmt.Errorf("siat solicitud cufd: cuis is required")
	}
	if s.CodigoModalidad <= 0 {
		return fmt.Errorf("siat solicitud cufd: codigoModalidad is required")
	}
	if s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat solicitud cufd: codigoPuntoVenta must be >= 0")
	}
	return nil
}
