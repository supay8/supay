package siat

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// XMLDateTime es una fecha/hora del SIAT que puede serializarse a JSON.
type XMLDateTime struct {
	time.Time
}

func (d XMLDateTime) MarshalJSON() ([]byte, error) {
	if d.Time.IsZero() {
		return []byte("null"), nil
	}
	return json.Marshal(d.Time.Format(time.RFC3339Nano))
}

func (d *XMLDateTime) UnmarshalJSON(data []byte) error {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "null" || trimmed == "" {
		d.Time = time.Time{}
		return nil
	}

	var value string
	if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
		return err
	}

	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
	} {
		if parsed, err := time.ParseInLocation(layout, value, LaPaz); err == nil {
			d.Time = parsed
			return nil
		}
	}

	return fmt.Errorf("siat: formato de fecha no soportado %q", value)
}

func (d XMLDateTime) String() string {
	if d.Time.IsZero() {
		return ""
	}
	return d.Time.Format(time.RFC3339Nano)
}

// Mensaje es un mensaje de respuesta devuelto por el SIAT.
type Mensaje struct {
	Codigo      int    `json:"codigo"`
	Descripcion string `json:"descripcion"`
}

// SolicitudCuis son los datos por-solicitud para obtener un CUIS.
type SolicitudCuis struct {
	CodigoAmbiente   int     `json:"codigoAmbiente"`
	CodigoSistema    string  `json:"codigoSistema"`
	Nit              string  `json:"nit"`
	CodigoSucursal   int     `json:"codigoSucursal"`
	CodigoModalidad  int     `json:"codigoModalidad"`
	CodigoPuntoVenta int     `json:"codigoPuntoVenta"`
	Cuis             *string `json:"cuis,omitempty"`
}

// RespuestaCuis es la respuesta del SIAT para la solicitud de CUIS.
type RespuestaCuis struct {
	Codigo        string      `json:"codigo"`
	FechaVigencia XMLDateTime `json:"fechaVigencia"`
	Transaccion   bool        `json:"transaccion"`
	Mensajes      []Mensaje   `json:"mensajes,omitempty"`
}

// SolicitudCufd son los datos por-solicitud para obtener un CUFD.
type SolicitudCufd struct {
	CodigoAmbiente   int    `json:"codigoAmbiente"`
	CodigoSistema    string `json:"codigoSistema"`
	Nit              string `json:"nit"`
	CodigoSucursal   int    `json:"codigoSucursal"`
	CodigoModalidad  int    `json:"codigoModalidad"`
	CodigoPuntoVenta int    `json:"codigoPuntoVenta"`
	Cuis             string `json:"cuis"`
}

// RespuestaCufd es la respuesta del SIAT para la solicitud de CUFD.
// Nota: el SDK go-siat v2 no expone codigoQR; se mantiene el campo por
// compatibilidad del modelo de persistencia (queda nil).
type RespuestaCufd struct {
	Codigo        string      `json:"codigo"`
	CodigoControl string      `json:"codigoControl"`
	CodigoQR      *string     `json:"codigoQR,omitempty"`
	Direccion     string      `json:"direccion"`
	FechaVigencia XMLDateTime `json:"fechaVigencia"`
	Transaccion   bool        `json:"transaccion"`
	Mensajes      []Mensaje   `json:"mensajes,omitempty"`
}

func (s SolicitudCuis) Validate() error {
	if s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat solicitud cuis: codigoAmbiente inválido")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat solicitud cuis: codigoSistema es obligatorio")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat solicitud cuis: nit es obligatorio")
	}
	if s.CodigoSucursal < 0 {
		return fmt.Errorf("siat solicitud cuis: codigoSucursal debe ser >= 0")
	}
	if s.CodigoModalidad <= 0 {
		return fmt.Errorf("siat solicitud cuis: codigoModalidad es obligatorio")
	}
	if s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat solicitud cuis: codigoPuntoVenta debe ser >= 0")
	}
	return nil
}

func (s SolicitudCufd) Validate() error {
	if s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat solicitud cufd: codigoAmbiente inválido")
	}
	if strings.TrimSpace(s.CodigoSistema) == "" {
		return fmt.Errorf("siat solicitud cufd: codigoSistema es obligatorio")
	}
	if strings.TrimSpace(s.Nit) == "" {
		return fmt.Errorf("siat solicitud cufd: nit es obligatorio")
	}
	if s.CodigoSucursal < 0 {
		return fmt.Errorf("siat solicitud cufd: codigoSucursal debe ser >= 0")
	}
	if strings.TrimSpace(s.Cuis) == "" {
		return fmt.Errorf("siat solicitud cufd: cuis es obligatorio")
	}
	if s.CodigoModalidad <= 0 {
		return fmt.Errorf("siat solicitud cufd: codigoModalidad es obligatorio")
	}
	if s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat solicitud cufd: codigoPuntoVenta debe ser >= 0")
	}
	return nil
}
