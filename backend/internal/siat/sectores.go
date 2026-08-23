package siat

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// TipoFacturaDocumento es el catálogo tipoFacturaDocumento del SIAT (2 bits del
// CUF y campo tipoFacturaDocumento de recepcionFactura): debe ser el MISMO valor
// en ambos lugares o el SIAT rechaza el documento.
const (
	// TipoDocumentoFacturaConCredito: facturas con derecho a crédito fiscal
	// (sectores marcados "Con" en el catálogo normativo).
	TipoDocumentoFacturaConCredito = 1
	// TipoDocumentoFacturaSinCredito: facturas sin derecho a crédito fiscal
	// (sectores "Sin") y documentos equivalentes (boleto aéreo).
	TipoDocumentoFacturaSinCredito = 2
	// TipoDocumentoNotaCreditoDebito: notas de crédito/débito y documentos de
	// ajuste (sectores 24, 29, 47 y 48).
	TipoDocumentoNotaCreditoDebito = 3
)

// OperacionDocumento indica qué servicio SOAP del SIAT recibe el documento:
// las facturas van por recepcionFactura (y paquete/masiva); los documentos de
// ajuste (notas) por recepcionDocumentoAjuste.
type OperacionDocumento int

const (
	OperacionRecepcionFactura OperacionDocumento = iota
	OperacionDocumentoAjuste
)

func (o OperacionDocumento) String() string {
	if o == OperacionDocumentoAjuste {
		return "documento_ajuste"
	}
	return "recepcion_factura"
}

// SectorLayout identifies the XML model when a document-sector has variants.
type SectorLayout string

const (
	LayoutNotaCreditoDebito       SectorLayout = "nota_credito_debito"
	LayoutNotaFiscalCreditoDebito SectorLayout = "nota_fiscal_credito_debito"
)

// FachadaSDK identifica la fachada del SDK go-siat que atiende al sector. Cada
// fachada es un endpoint SOAP distinto del SIAT; enviar un sector por la fachada
// equivocada produce el rechazo 932 (CODIGO DOCUMENTO SECTOR NO CORRESPONDE AL
// SERVICIO).
type FachadaSDK int

// String devuelve el nombre legible de la fachada (para logs y API).
func (f FachadaSDK) String() string {
	switch f {
	case FachadaCompraVenta:
		return "compra_venta"
	case FachadaTelecomunicaciones:
		return "telecomunicaciones"
	case FachadaServicioBasico:
		return "servicio_basico"
	case FachadaEntidadFinanciera:
		return "entidad_financiera"
	case FachadaBoletoAereo:
		return "boleto_aereo"
	case FachadaDocumentoAjuste:
		return "documento_ajuste"
	default:
		return "por_modalidad"
	}
}

const (
	FachadaPorModalidad       FachadaSDK = iota // Electronica()/Computarizada() según modalidad
	FachadaCompraVenta                          // sectores 1, 35, 41
	FachadaTelecomunicaciones                   // sectores 22, 49
	FachadaServicioBasico                       // sectores 13, 40
	FachadaEntidadFinanciera                    // sector 15
	FachadaBoletoAereo                          // sector 30 (sin recepción individual ni paquetes)
	FachadaDocumentoAjuste                      // sectores 24, 29, 47 y 48
)

// Códigos de documento-sector con tratamiento especial en el flujo de Supay.
const (
	SectorCompraVenta       = 1
	SectorTasaCero          = 8
	SectorEducativo         = 11
	SectorNotaCreditoDebito = 24
)

// CampoSector declara un campo específico de un documento-sector: su clave JSON
// dentro de datos_sector, el método With* del builder de cabecera donde se
// aplica y su tipo esperado. La misma declaración alimenta la validación
// fail-fast en POST /invoices y la metadata de GET /invoices/sectores.
type CampoSector struct {
	JSON      string `json:"clave"`
	Metodo    string `json:"-"`
	Tipo      string `json:"tipo"` // string | int | float | fecha
	Requerido bool   `json:"requerido"`
}

// SectorProfile describe cómo emitir un documento-sector del SIAT: metadatos
// normativos, fábricas de builders del SDK y sus campos específicos. Agregar un
// sector nuevo al sistema = agregar una entrada a catalogoSectores.
type SectorProfile struct {
	Codigo               int                `json:"codigo_documento_sector"`
	Nombre               string             `json:"nombre"`
	Layout               string             `json:"layout,omitempty"`
	TipoFacturaDocumento int                `json:"tipo_factura_documento"`
	Operacion            OperacionDocumento `json:"operacion"`
	Fachada              FachadaSDK         `json:"-"`
	ConDetalle           bool               `json:"con_detalle"`
	// DetalleUnico marca los sectores prevalorados (23, 36): su XSD acepta una
	// sola línea, que se envía con WithDetalle en lugar de AddDetalle.
	DetalleUnico bool `json:"detalle_unico,omitempty"`
	Experimental bool `json:"experimental"`
	// MontoSujetoIvaCero marca los sectores donde montoTotalSujetoIva se envía
	// en 0 (tasa cero); en el resto viaja igual al monto total.
	MontoSujetoIvaCero bool          `json:"-"`
	Campos             []CampoSector `json:"campos_especificos"`
	// Modalidades limita las modalidades habilitadas para el sector. Un perfil
	// vacío acepta ambas modalidades, que es el comportamiento del SDK actual.
	Modalidades []int          `json:"modalidades,omitempty"`
	Facade      FacadeSelector `json:"-"`

	builders buildersSector `json:"-"`
	adapter  SectorAdapter  `json:"-"`
}

// SectorDocument es la representación interna común entre validación y
// construcción. Values contiene datos normalizados para el builder; Present y
// Null mantienen la diferencia entre campo ausente y campo JSON explícitamente
// nulo para que los adaptadores puedan aplicar reglas nilables.
type SectorDocument struct {
	Request SolicitudFactura
	Values  map[string]any
	Present map[string]bool
	Null    map[string]bool
	Payload any
}

// SectorAdapter encapsula las reglas de un documento-sector. Los builders del
// SDK no comparten una interfaz Go, por eso el adaptador es el límite estable
// de nuestra aplicación y oculta esa variación.
type SectorAdapter interface {
	Prepare(*SectorProfile, SolicitudFactura) (SectorDocument, error)
	Build(*SectorProfile, SectorDocument, string) any
}

// buildersSector agrupa las fábricas de builders del SDK para un sector. Las
// tres siguen el patrón jerárquico del SDK (factura raíz / cabecera / detalle);
// detalle es nil en los sectores sin líneas (prevaloradas y boleto aéreo).
type buildersSector struct {
	factura  func(modalidad int) any
	cabecera func() any
	detalle  func() any
}

// FacadeSelector makes the endpoint choice visible while keeping modality
// selection as a separate decision.
type FacadeSelector struct {
	name        string
	fixed       FachadaSDK
	byModalidad bool
}

func FacadeFija(nombre string) FacadeSelector {
	for _, fachada := range []FachadaSDK{
		FachadaCompraVenta, FachadaTelecomunicaciones, FachadaServicioBasico,
		FachadaEntidadFinanciera, FachadaBoletoAereo, FachadaDocumentoAjuste,
	} {
		if fachada.String() == nombre {
			return FacadeSelector{name: nombre, fixed: fachada}
		}
	}
	return FacadeSelector{name: nombre}
}

func FacadePorModalidad() FacadeSelector {
	return FacadeSelector{name: "por_modalidad", fixed: FachadaPorModalidad, byModalidad: true}
}

func (f FacadeSelector) String() string {
	if f.name != "" {
		return f.name
	}
	return f.fixed.String()
}

func (f FacadeSelector) IsByModalidad() bool { return f.byModalidad }

func (f FacadeSelector) Fixed() FachadaSDK { return f.fixed }

type SectorKey struct {
	Codigo int
	Layout string
}

// SectorRegistry stores opaque SDK builder factories. Layout is part of the
// key because sector 24 intentionally has two document models.
type SectorRegistry struct {
	entries map[SectorKey]*SectorProfile
}

var sectorRegistry *SectorRegistry
var registroSectores map[int]*SectorProfile

func init() {
	sectorRegistry = &SectorRegistry{entries: make(map[SectorKey]*SectorProfile, len(catalogoSectores))}
	registroSectores = make(map[int]*SectorProfile, len(catalogoSectores))
	for _, p := range catalogoSectores {
		key := SectorKey{Codigo: p.Codigo, Layout: p.Layout}
		if _, duplicado := sectorRegistry.entries[key]; duplicado {
			panic(fmt.Sprintf("siat sectores: código %d layout %q registrado dos veces", p.Codigo, p.Layout))
		}
		if p.Codigo == SectorCompraVenta {
			p.adapter = compraVentaAdapter{}
		} else if p.adapter == nil {
			p.adapter = genericSectorAdapter{}
		}
		p.Facade = facadeSelectorFor(p.Fachada)
		sectorRegistry.entries[key] = p
		if _, exists := registroSectores[p.Codigo]; !exists {
			registroSectores[p.Codigo] = p
		}
	}
}

func facadeSelectorFor(fachada FachadaSDK) FacadeSelector {
	if fachada == FachadaPorModalidad {
		return FacadePorModalidad()
	}
	return FacadeFija(fachada.String())
}

func (p *SectorProfile) ValidarModalidad(modalidad int) error {
	if modalidad != ModalidadElectronica && modalidad != ModalidadComputarizada {
		return fmt.Errorf("siat sectores %d (%s): modalidad %d no es válida", p.Codigo, p.Nombre, modalidad)
	}
	if len(p.Modalidades) == 0 {
		return nil
	}
	for _, permitida := range p.Modalidades {
		if modalidad == permitida {
			return nil
		}
	}
	return fmt.Errorf("siat sectores %d (%s): modalidad %d no está habilitada", p.Codigo, p.Nombre, modalidad)
}

// PerfilSector devuelve el perfil del documento-sector indicado. Un código no
// registrado (p.ej. los inexistentes 25-27, 32 o el 33 sin builder) produce
// error antes de llegar al SIAT (que lo rechazaría con 931).
func PerfilSector(codigo int) (*SectorProfile, error) {
	if codigo == SectorNotaCreditoDebito {
		return nil, fmt.Errorf("el sector %d tiene múltiples layouts; especifique uno de %q o %q", codigo, LayoutNotaCreditoDebito, LayoutNotaFiscalCreditoDebito)
	}
	return PerfilSectorLayout(codigo, "")
}

func PerfilSectorLayout(codigo int, layout string) (*SectorProfile, error) {
	if codigo <= 0 {
		codigo = SectorCompraVenta
	}
	if codigo == SectorNotaCreditoDebito && strings.TrimSpace(layout) == "" {
		return nil, fmt.Errorf("el sector %d tiene múltiples layouts; especifique uno de %q o %q", codigo, LayoutNotaCreditoDebito, LayoutNotaFiscalCreditoDebito)
	}
	if layout == "" {
		if p, ok := registroSectores[codigo]; ok {
			return p, nil
		}
	}
	p, ok := sectorRegistry.entries[SectorKey{Codigo: codigo, Layout: layout}]
	if !ok {
		return nil, fmt.Errorf("siat sectores: el documento-sector %d layout %q no está soportado; consulte los perfiles disponibles", codigo, layout)
	}
	return p, nil
}

// PerfilesSector lista todos los perfiles registrados ordenados por código
// (metadata para GET /invoices/sectores).
func PerfilesSector() []*SectorProfile {
	out := make([]*SectorProfile, 0, len(sectorRegistry.entries))
	for _, p := range sectorRegistry.entries {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Codigo == out[j].Codigo {
			return out[i].Layout < out[j].Layout
		}
		return out[i].Codigo < out[j].Codigo
	})
	return out
}

// TipoDocumentoResuelto deriva el tipoFacturaDocumento del perfil; un override
// explícito (>0) del cliente/catálogo tiene prioridad.
func (p *SectorProfile) TipoDocumentoResuelto(override int) int {
	if override > 0 {
		return override
	}
	if p.TipoFacturaDocumento <= 0 {
		return TipoDocumentoFacturaConCredito
	}
	return p.TipoFacturaDocumento
}

// EsAjuste indica si el sector es un documento de ajuste (nota), que viaja por
// el servicio DocumentoAjuste del SIAT.
func (p *SectorProfile) EsAjuste() bool {
	return p.Operacion == OperacionDocumentoAjuste
}

func (p *SectorProfile) HasBuilder() bool {
	return p.builders.factura != nil && p.builders.cabecera != nil
}

// PrepararDatosSector es el punto de entrada para las capas superiores: valida
// datos_sector aplicando antes la fusión de los campos legados educativos
// (nombre_estudiante/periodo_facturado) cuando el perfil corresponde (11/46).
func (p *SectorProfile) PrepararDatosSector(req SolicitudFactura) (map[string]any, error) {
	doc, err := p.adapter.Prepare(p, req)
	if err != nil {
		return nil, err
	}
	return doc.Values, nil
}

// ValidarDatosSector decodifica y valida los datos específicos del sector
// contra la declaración de Campos del perfil: verifica tipos y campos
// requeridos. Devuelve los valores normalizados (string/int64/float64/time.Time)
// listos para aplicarse sobre el builder de cabecera.
func (p *SectorProfile) ValidarDatosSector(datos json.RawMessage) (map[string]any, error) {
	brutos := map[string]any{}
	if len(datos) > 0 {
		if err := json.Unmarshal(datos, &brutos); err != nil {
			return nil, fmt.Errorf("siat sectores %d: datos_sector no es un objeto JSON válido: %w", p.Codigo, err)
		}
	}
	desconocidos := map[string]bool{}
	for k := range brutos {
		desconocidos[k] = true
	}
	valores := make(map[string]any, len(p.Campos))
	var faltantes []string
	for _, campo := range p.Campos {
		delete(desconocidos, campo.JSON)
		crudo, presente := brutos[campo.JSON]
		valor, err := normalizarValorCampo(p.Codigo, campo, crudo, presente)
		if err != nil {
			return nil, err
		}
		if !presente || valor == nil {
			if campo.Requerido {
				faltantes = append(faltantes, campo.JSON)
			}
			continue
		}
		valores[campo.JSON] = valor
	}
	if len(faltantes) > 0 {
		sort.Strings(faltantes)
		return nil, fmt.Errorf("siat sectores %d (%s): faltan campos obligatorios en datos_sector: %s",
			p.Codigo, p.Nombre, strings.Join(faltantes, ", "))
	}
	if len(desconocidos) > 0 {
		claves := make([]string, 0, len(desconocidos))
		for k := range desconocidos {
			claves = append(claves, k)
		}
		sort.Strings(claves)
		return nil, fmt.Errorf("siat sectores %d (%s): claves no reconocidas en datos_sector: %s",
			p.Codigo, p.Nombre, strings.Join(claves, ", "))
	}
	return valores, nil
}

func normalizarValorCampo(codigo int, campo CampoSector, crudo any, presente bool) (any, error) {
	if !presente || crudo == nil {
		return nil, nil
	}
	switch campo.Tipo {
	case "string":
		s, ok := crudo.(string)
		if !ok {
			return nil, fmt.Errorf("siat sectores %d: datos_sector.%s debe ser string", codigo, campo.JSON)
		}
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, nil
		}
		return s, nil
	case "int":
		f, ok := toFloat(crudo)
		if !ok {
			return nil, fmt.Errorf("siat sectores %d: datos_sector.%s debe ser numérico entero", codigo, campo.JSON)
		}
		return int64(f), nil
	case "float":
		f, ok := toFloat(crudo)
		if !ok {
			return nil, fmt.Errorf("siat sectores %d: datos_sector.%s debe ser numérico", codigo, campo.JSON)
		}
		return f, nil
	case "fecha":
		s, ok := crudo.(string)
		if !ok {
			return nil, fmt.Errorf("siat sectores %d: datos_sector.%s debe ser fecha (YYYY-MM-DD o RFC3339)", codigo, campo.JSON)
		}
		for _, layout := range []string{"2006-01-02T15:04:05Z07:00", "2006-01-02T15:04:05", "2006-01-02"} {
			if t, err := time.ParseInLocation(layout, strings.TrimSpace(s), LaPaz); err == nil {
				return t, nil
			}
		}
		return nil, fmt.Errorf("siat sectores %d: datos_sector.%s tiene formato de fecha inválido (%q)", codigo, campo.JSON, s)
	default:
		return nil, fmt.Errorf("siat sectores %d: campo %s declara tipo desconocido %q", codigo, campo.JSON, campo.Tipo)
	}
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		if f, err := n.Float64(); err == nil {
			return f, true
		}
	}
	return 0, false
}
