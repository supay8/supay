package siat

import (
	"encoding/json"
	"fmt"
	"sort"

	"github.com/brandsrx/supay/internal/domain/fiscal"
)

type CampoSector = fiscal.CampoSector
type SectorLayout = fiscal.SectorLayout
type OperacionDocumento = fiscal.OperacionDocumento
type FachadaSDK = fiscal.FachadaSDK
type FacadeSelector = fiscal.FacadeSelector
type SectorKey = fiscal.SectorKey

const (
	TipoDocumentoFacturaConCredito = fiscal.TipoDocumentoFacturaConCredito
	TipoDocumentoFacturaSinCredito = fiscal.TipoDocumentoFacturaSinCredito
	TipoDocumentoNotaCreditoDebito = fiscal.TipoDocumentoNotaCreditoDebito
	OperacionRecepcionFactura      = fiscal.OperacionRecepcionFactura
	OperacionDocumentoAjuste       = fiscal.OperacionDocumentoAjuste
	LayoutNotaCreditoDebito        = fiscal.LayoutNotaCreditoDebito
	LayoutNotaFiscalCreditoDebito  = fiscal.LayoutNotaFiscalCreditoDebito
	FachadaPorModalidad            = fiscal.FachadaPorModalidad
	FachadaCompraVenta             = fiscal.FachadaCompraVenta
	FachadaTelecomunicaciones      = fiscal.FachadaTelecomunicaciones
	FachadaServicioBasico          = fiscal.FachadaServicioBasico
	FachadaEntidadFinanciera       = fiscal.FachadaEntidadFinanciera
	FachadaBoletoAereo             = fiscal.FachadaBoletoAereo
	FachadaDocumentoAjuste         = fiscal.FachadaDocumentoAjuste
	SectorCompraVenta              = fiscal.SectorCompraVenta
	SectorTasaCero                 = fiscal.SectorTasaCero
	SectorEducativo                = fiscal.SectorEducativo
	SectorNotaCreditoDebito        = fiscal.SectorNotaCreditoDebito
)

type SectorProfile struct {
	*fiscal.SectorProfile
	builders buildersSector
	adapter  SectorAdapter
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

type SectorRegistry struct{ entries map[SectorKey]*SectorProfile }

var sectorRegistry *SectorRegistry
var registroSectores map[int]*SectorProfile

func init() {
	sectorRegistry = &SectorRegistry{entries: make(map[SectorKey]*SectorProfile)}
	registroSectores = make(map[int]*SectorProfile)
	for _, p := range catalogoSectores {
		core, err := fiscal.PerfilSectorLayout(p.Codigo, p.Layout)
		if err != nil {
			panic(err)
		}
		p.SectorProfile = core
		if p.Codigo == SectorCompraVenta {
			p.adapter = compraVentaAdapter{}
		} else {
			p.adapter = genericSectorAdapter{}
		}
		key := SectorKey{Codigo: p.Codigo, Layout: p.Layout}
		if _, ok := sectorRegistry.entries[key]; ok {
			panic(fmt.Sprintf("perfil duplicado: %v", key))
		}
		sectorRegistry.entries[key] = p
		if _, ok := registroSectores[p.Codigo]; !ok {
			registroSectores[p.Codigo] = p
		}
	}
}
func FacadePorModalidad() FacadeSelector    { return fiscal.FacadePorModalidad() }
func FacadeFija(name string) FacadeSelector { return fiscal.FacadeFija(name) }
func PerfilSector(code int) (*SectorProfile, error) {
	if _, err := fiscal.PerfilSector(code); err != nil {
		return nil, err
	}
	return PerfilSectorLayout(code, "")
}
func PerfilSectorLayout(code int, layout string) (*SectorProfile, error) {
	core, err := fiscal.PerfilSectorLayout(code, layout)
	if err != nil {
		return nil, err
	}
	return sectorRegistry.entries[SectorKey{Codigo: core.Codigo, Layout: core.Layout}], nil
}
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
func (p *SectorProfile) HasBuilder() bool {
	return p.builders.factura != nil && p.builders.cabecera != nil
}
func (p *SectorProfile) PrepararDatosSector(req SolicitudFactura) (map[string]any, error) {
	return p.SectorProfile.PrepararDatosSector(fiscal.SolicitudFactura{DatosSector: req.DatosSector, NombreEstudiante: req.NombreEstudiante, PeriodoFacturado: req.PeriodoFacturado})
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
