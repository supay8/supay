package siat

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"reflect"
	"strings"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
	"github.com/ron86i/go-siat/v2/pkg/models"
	"github.com/ron86i/go-siat/v2/pkg/utils"
)

// EmisionPaqueteOffline es el codigoEmision que exige el SIAT para el envío de
// paquetes: emisión fuera de línea (contingencia).
const EmisionPaqueteOffline = goSiat.EmisionOffline

// MaxFacturasPorPaquete es el límite de facturas por paquete definido por el SIN.
const MaxFacturasPorPaquete = 500

// SolicitudPaqueteFactura agrupa los prerrequisitos para enviar un paquete de
// facturas al SIAT (recepcionPaqueteFactura), usado en emisión por lotes o en
// contingencia (codigoEmision = EmisionOffline). Todas las facturas del paquete
// deben pertenecer al mismo documento-sector y compartir el mismo CUIS/CUFD.
type SolicitudPaqueteFactura struct {
	CodigoAmbiente   int    `json:"codigoAmbiente"`
	CodigoSistema    string `json:"codigoSistema"`
	Nit              string `json:"nit"`
	Modalidad        int    `json:"modalidad"`
	CodigoSucursal   int    `json:"codigoSucursal"`
	CodigoPuntoVenta int    `json:"codigoPuntoVenta"`
	Cuis             string `json:"cuis"`
	Cufd             string `json:"cufd"`
	CodigoControl    string `json:"codigoControl"`

	// CodigoDocumentoSector es el diseño de factura del paquete (1 = compraventa,
	// 11 = sector educativo). Todas las facturas deben ser del mismo sector.
	CodigoDocumentoSector int    `json:"codigoDocumentoSector"`
	Layout                string `json:"layout,omitempty"`
	// CodigoTipoFactura es el tipo de documento factura (1 = factura).
	CodigoTipoFactura int `json:"codigoTipoFactura"`
	// CodigoEmision es el tipo de emisión (1 = en línea, 2 = fuera de línea /
	// contingencia). Para paquetes el SIAT exige 2 (EmisionPaqueteOffline).
	CodigoEmision int `json:"codigoEmision"`
	// CodigoEvento es el código de recepción del evento significativo registrado
	// (respuesta de registroEventoSignificativo) que motiva el envío del paquete.
	CodigoEvento int64  `json:"codigoEvento"`
	Archivo      string `json:"archivo,omitempty"`
	HashArchivo  string `json:"hashArchivo,omitempty"`
	// Descripcion describe la contingencia que originó el paquete (p.ej. "CORTE
	// DEL SERVICIO DE INTERNET"). Es solo para trazabilidad: el SIAT no recibe
	// este campo en recepcionPaqueteFactura, la descripción ya quedó registrada
	// con el evento significativo.
	Descripcion string `json:"descripcion,omitempty"`

	// Facturas son las facturas del paquete (máximo 500). Cada una conserva los
	// datos de cliente/ítems mapeados a catálogos SIN; la identidad del paquete
	// (ambiente, sistema, NIT, modalidad, sucursal, punto de venta, CUIS/CUFD y
	// código de control) se hereda a las facturas que no la traigan.
	Facturas []SolicitudFactura `json:"facturas"`
}

// ResultadoPaquete es la respuesta procesada de recepcionPaqueteFactura y de
// validacionRecepcionPaqueteFactura.
type ResultadoPaquete struct {
	Transaccion     bool      `json:"transaccion"`
	CodigoEstado    int       `json:"codigo_estado"`
	CodigoRecepcion string    `json:"codigo_recepcion,omitempty"`
	Mensajes        []Mensaje `json:"mensajes,omitempty"`

	// Archivo es la cadena Base64 del TAR.GZ del paquete tal como se envió
	// (auditoría).
	Archivo string `json:"archivo,omitempty"`
	// HashArchivo es el hash SHA-256 del archivo comprimido del paquete.
	HashArchivo string `json:"hash_archivo,omitempty"`
	// CantidadFacturas es el número de facturas empaquetadas y enviadas.
	CantidadFacturas int `json:"cantidad_facturas"`
	// Cufs contiene el CUF de cada factura del paquete, para poder consultar o
	// anular individualmente cada documento después del envío.
	Cufs []string `json:"cufs,omitempty"`
}

// EnviarPaqueteFactura envía un paquete de facturas al SIAT
// (recepcionPaqueteFactura). Los XML persistidos se empaquetan sin modificar sus
// bytes, firmas ni CUF. Para documentos nuevos se utilizan los builders y la
// firma del SDK con emisión offline. El CodigoRecepcion devuelto se usa luego
// en ValidarPaqueteFactura.
func (s *Service) EnviarPaqueteFactura(ctx context.Context, req SolicitudPaqueteFactura) (*ResultadoPaquete, error) {
	prepared, err := s.prepararPaquete(ctx, req)
	if err != nil {
		return nil, err
	}
	return s.enviarPaquetePreparada(ctx, prepared)
}

type paquetePreparada struct {
	service *Service
	req     SolicitudPaqueteFactura
	perfil  *SectorProfile
	request models.RecepcionPaqueteFactura
	result  ResultadoPaquete
}

func (s *Service) prepararPaquete(ctx context.Context, req SolicitudPaqueteFactura) (*paquetePreparada, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.sdk == nil {
		return nil, fmt.Errorf("siat paquete: servicio SIAT no inicializado")
	}
	if err := applyIdentityValues(s.sdk.Config(), &req.CodigoAmbiente, &req.CodigoSistema, &req.Nit); err != nil {
		return nil, err
	}
	req = req.normalized()
	if err := req.validate(); err != nil {
		return nil, err
	}

	perfil, err := PerfilSectorLayout(req.sector(), req.Layout)
	if err != nil {
		return nil, fmt.Errorf("siat paquete: %w", err)
	}
	if perfil.Facade.Fixed() == FachadaBoletoAereo {
		return nil, fmt.Errorf("siat paquete: el boleto aéreo no admite paquete de facturas; use la emisión masiva")
	}
	if perfil.EsAjuste() {
		return nil, fmt.Errorf("siat paquete: los documentos de ajuste (sectores 24/29/47/48) no se envían en paquete")
	}
	tipoFactura := perfil.TipoDocumentoResuelto(req.CodigoTipoFactura)
	codigoEmision := req.codigoEmision()

	facturas := make([]any, 0, len(req.Facturas))
	cufs := make([]string, 0, len(req.Facturas))
	prepared := req.hasPersistedXML()
	if perfil.HasBuilder() && !prepared {
		for i := range req.Facturas {
			if err := applyIdentityValues(s.sdk.Config(), &req.Facturas[i].CodigoAmbiente, &req.Facturas[i].CodigoSistema, &req.Facturas[i].Nit); err != nil {
				return nil, fmt.Errorf("siat paquete factura %d: %w", i+1, err)
			}
			factura, cuf, _, err := buildFacturaSDK(req.Facturas[i], codigoEmision)
			if err != nil {
				return nil, fmt.Errorf("siat paquete factura %d: %w", i+1, err)
			}
			facturas = append(facturas, factura)
			cufs = append(cufs, cuf)
		}
	}

	paquete := models.NewRecepcionPaqueteFacturaBuilder().
		WithCodigoModalidad(req.Modalidad).
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(perfil.Codigo).
		WithCodigoEmision(codigoEmision).
		WithTipoFacturaDocumento(tipoFactura).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).

		// El SIAT interpreta la hora de pared sin zona de fechaEnvio como hora
		// local de Bolivia (UTC-4); el SDK formatea la hora tal cual la recibe
		// (no convierte), por lo que se envía la hora de pared de La Paz.
		WithFechaEnvio(time.Now().In(LaPaz)).
		WithCodigoEvento(req.CodigoEvento).

		// Cafc es Nilable en el SDK y, si queda en nil, emite <cafc xsi:nil="true"/>
		// con el prefijo xsi sin declarar en el envelope del request, lo que hace
		// que el SIAT rechace el paquete con "Undeclared namespace prefix". Se
		// envía vacío para evitar el xsi:nil.
		WithCafc(&emptyStr)

	if prepared {
		archivo, hash, err := empaquetarXMLPersistidos(req.Facturas)
		if err != nil {
			return nil, fmt.Errorf("siat paquete: %w", err)
		}
		paquete.WithArchivo(archivo).WithHashArchivo(hash).WithCantidadFacturas(len(req.Facturas))
		for _, factura := range req.Facturas {
			cufs = append(cufs, factura.Cuf)
		}
	} else if perfil.HasBuilder() {
		if err := paquete.WithFacturas(facturas, s.sdk.Config()); err != nil {
			return nil, fmt.Errorf("siat paquete: no se pudo empaquetar las facturas: %w", err)
		}
	} else {
		paquete.WithArchivo(req.Archivo).WithHashArchivo(req.HashArchivo).WithCantidadFacturas(len(req.Facturas))
	}

	built := paquete.Build()
	archivo, hash, cantidad := extraerArchivoPaquete(built)

	if perfil.HasBuilder() || len(cufs) > 0 {
		if err := recuperarDocumentosLote(archivo, req.Facturas, cufs); err != nil {
			return nil, fmt.Errorf("siat paquete: %w", err)
		}
	}
	req.Archivo, req.HashArchivo = archivo, hash
	return &paquetePreparada{
		service: s, req: req, perfil: perfil, request: built,
		result: ResultadoPaquete{Archivo: archivo, HashArchivo: hash, CantidadFacturas: cantidad, Cufs: cufs},
	}, nil
}

func (s *Service) enviarPaquetePreparada(ctx context.Context, prepared *paquetePreparada) (*ResultadoPaquete, error) {
	req, perfil, built := prepared.req, prepared.perfil, prepared.request
	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.paqueteParaPerfil(ctx, perfil, req.Modalidad, built)
	if err != nil {
		return nil, fmt.Errorf("siat paquete: %w", err)
	}

	// Nota: RespuestaRecepcion no implementa common.Result, por lo que
	// goSiat.Verify no aplica; la verificación es manual (Transaccion/CodigoEstado).
	transaccion, codigoEstado, codigoRecepcion, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat paquete: %w", err)
	}

	result := prepared.result
	result.Transaccion, result.CodigoEstado = transaccion, codigoEstado
	result.CodigoRecepcion, result.Mensajes = codigoRecepcion, mensajes
	return &result, nil
}

// ValidarPaqueteFactura consulta al SIAT la validación de un paquete ya enviado
// (validacionRecepcionPaqueteFactura) usando el CodigoRecepcion devuelto por
// EnviarPaqueteFactura.
func (s *Service) ValidarPaqueteFactura(ctx context.Context, req SolicitudPaqueteFactura, codigoRecepcion string) (*ResultadoPaquete, error) {
	if s.sdk == nil {
		return nil, fmt.Errorf("siat paquete: servicio SIAT no inicializado")
	}
	if err := applyIdentityValues(s.sdk.Config(), &req.CodigoAmbiente, &req.CodigoSistema, &req.Nit); err != nil {
		return nil, err
	}
	if err := req.validateBase(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(codigoRecepcion) == "" {
		return nil, fmt.Errorf("siat paquete: codigoRecepcion es obligatorio para validar el paquete")
	}
	perfil, err := PerfilSectorLayout(req.sector(), req.Layout)
	if err != nil {
		return nil, fmt.Errorf("siat paquete: %w", err)
	}
	request := models.NewValidacionRecepcionPaqueteFacturaBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoDocumentoSector(perfil.Codigo).
		WithCodigoEmision(req.codigoEmision()).
		WithTipoFacturaDocumento(perfil.TipoDocumentoResuelto(req.CodigoTipoFactura)).
		WithCuis(req.Cuis).
		WithCufd(req.Cufd).
		WithCodigoRecepcion(codigoRecepcion).
		WithCodigoModalidad(req.Modalidad).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.validacionPaqueteParaPerfil(ctx, perfil, req.Modalidad, request)
	if err != nil {
		return nil, fmt.Errorf("siat paquete: %w", err)
	}

	transaccion, codigoEstado, codigoRecepcionResp, mensajes, err := extraerResultadoFacturacion(resp)
	if err != nil {
		return nil, fmt.Errorf("siat paquete: %w", err)
	}

	return &ResultadoPaquete{
		Transaccion:     transaccion,
		CodigoEstado:    codigoEstado,
		CodigoRecepcion: codigoRecepcionResp,
		Mensajes:        mensajes,
	}, nil
}

// normalized devuelve una copia de la solicitud en la que cada factura hereda
// la identidad común del paquete (ambiente, sistema, NIT, modalidad, sucursal,
// punto de venta, CUIS, CUFD, código de control y documento-sector) cuando no
// la trae propia.
func (s SolicitudPaqueteFactura) normalized() SolicitudPaqueteFactura {
	out := s
	out.Facturas = make([]SolicitudFactura, len(s.Facturas))
	copy(out.Facturas, s.Facturas)
	for i := range out.Facturas {
		f := &out.Facturas[i]
		if f.CodigoAmbiente == 0 {
			f.CodigoAmbiente = s.CodigoAmbiente
		}
		if strings.TrimSpace(f.CodigoSistema) == "" {
			f.CodigoSistema = s.CodigoSistema
		}
		// Un XML emitido conserva su contexto histórico. Solo el sistema y el
		// ambiente del sobre pueden resolverse desde la configuración común.
		if f.XML != "" {
			continue
		}
		if strings.TrimSpace(f.Nit) == "" {
			f.Nit = s.Nit
		}
		if f.Modalidad == 0 {
			f.Modalidad = s.Modalidad
		}
		if f.CodigoSucursal == 0 {
			f.CodigoSucursal = s.CodigoSucursal
		}
		if f.CodigoPuntoVenta == 0 {
			f.CodigoPuntoVenta = s.CodigoPuntoVenta
		}
		if strings.TrimSpace(f.Cuis) == "" {
			f.Cuis = s.Cuis
		}
		if strings.TrimSpace(f.Cufd) == "" {
			f.Cufd = s.Cufd
		}
		if strings.TrimSpace(f.CodigoControl) == "" {
			f.CodigoControl = s.CodigoControl
		}
		if f.CodigoDocumentoSector == 0 {
			f.CodigoDocumentoSector = s.CodigoDocumentoSector
		}
		if strings.TrimSpace(f.Layout) == "" {
			f.Layout = s.Layout
		}
		if f.CodigoTipoFactura == 0 {
			f.CodigoTipoFactura = s.CodigoTipoFactura
		}
	}
	return out
}

// extraerArchivoPaquete lee archivo, hashArchivo y cantidadFacturas del request
// interno del SDK ya construido, para auditoría. El campo request del wrapper es
// no exportado, por lo que se lee con reflexión (String()/Int()).
func extraerArchivoPaquete(paquete models.RecepcionPaqueteFactura) (archivo, hash string, cantidad int) {
	v := reflect.ValueOf(paquete)
	wrapper := v.FieldByName("RequestWrapper")
	if !wrapper.IsValid() {
		return "", "", 0
	}
	request := wrapper.FieldByName("request")
	if !request.IsValid() || request.Kind() != reflect.Pointer {
		return "", "", 0
	}
	solicitud := request.Elem().FieldByName("SolicitudServicioRecepcionPaquete")
	if !solicitud.IsValid() {
		return "", "", 0
	}
	recep := solicitud.FieldByName("SolicitudRecepcionFactura")
	if !recep.IsValid() {
		return "", "", 0
	}
	if f := recep.FieldByName("Archivo"); f.IsValid() && f.Kind() == reflect.String {
		archivo = f.String()
	}
	if f := recep.FieldByName("HashArchivo"); f.IsValid() && f.Kind() == reflect.String {
		hash = f.String()
	}
	if f := solicitud.FieldByName("CantidadFacturas"); f.IsValid() && f.Kind() == reflect.Int {
		cantidad = int(f.Int())
	}
	return archivo, hash, cantidad
}

func (s SolicitudPaqueteFactura) sector() int {
	if s.CodigoDocumentoSector <= 0 {
		return SectorCompraVenta
	}
	return s.CodigoDocumentoSector
}

func (s SolicitudPaqueteFactura) codigoEmision() int {
	if s.CodigoEmision <= 0 {
		return EmisionPaqueteOffline
	}
	return s.CodigoEmision
}

// validateBase valida la identidad común del contribuyente que exigen tanto
// recepcionPaqueteFactura como validacionRecepcionPaqueteFactura.
func (s SolicitudPaqueteFactura) validateBase() error {
	if s.CodigoAmbiente != 0 && s.CodigoAmbiente != AmbienteProduccion && s.CodigoAmbiente != AmbientePruebas {
		return fmt.Errorf("siat paquete: codigoAmbiente inválido")
	}
	if s.Modalidad != ModalidadElectronica && s.Modalidad != ModalidadComputarizada {
		return fmt.Errorf("siat paquete: modalidad inválida (%d)", s.Modalidad)
	}
	if s.CodigoSucursal < 0 || s.CodigoPuntoVenta < 0 {
		return fmt.Errorf("siat paquete: codigoSucursal y codigoPuntoVenta deben ser >= 0")
	}
	if s.codigoEmision() != EmisionPaqueteOffline {
		return fmt.Errorf("siat paquete: codigoEmision debe ser %d", EmisionPaqueteOffline)
	}
	if strings.TrimSpace(s.Cuis) == "" || strings.TrimSpace(s.Cufd) == "" {
		return fmt.Errorf("siat paquete: cuis y cufd son obligatorios")
	}
	return nil
}

func (s SolicitudPaqueteFactura) validate() error {
	if err := s.validateBase(); err != nil {
		return err
	}
	if s.CodigoEvento <= 0 {
		return fmt.Errorf("siat paquete: codigoEvento es obligatorio (registre primero un evento significativo)")
	}
	if len(s.Facturas) == 0 {
		return fmt.Errorf("siat paquete: el paquete debe contener al menos una factura")
	}
	if len(s.Facturas) > MaxFacturasPorPaquete {
		return fmt.Errorf("siat paquete: el paquete supera el límite de %d facturas del SIN", MaxFacturasPorPaquete)
	}
	perfil, err := PerfilSectorLayout(s.sector(), s.Layout)
	if err != nil {
		return err
	}
	if err := perfil.ValidarModalidad(s.Modalidad); err != nil {
		return err
	}
	if err := validarIdentidadLote(s.Facturas, s, s.hasPersistedXML()); err != nil {
		return fmt.Errorf("siat paquete: %w", err)
	}
	if s.hasPersistedXML() {
		if s.Archivo != "" || s.HashArchivo != "" {
			return fmt.Errorf("siat paquete: no combine XML persistidos con un archivo de paquete")
		}
		for i, factura := range s.Facturas {
			if err := validarXMLPersistido(factura, perfil, s); err != nil {
				return fmt.Errorf("siat paquete factura %d: %w", i+1, err)
			}
		}
		return nil
	}
	if !perfil.HasBuilder() {
		if strings.TrimSpace(s.Archivo) == "" || strings.TrimSpace(s.HashArchivo) == "" {
			return fmt.Errorf("siat paquete sector %d: archivo y hashArchivo son obligatorios porque no existe builder", perfil.Codigo)
		}
		return nil
	}
	if strings.TrimSpace(s.Archivo) != "" || strings.TrimSpace(s.HashArchivo) != "" {
		return fmt.Errorf("siat paquete sector %d: archivo/hashArchivo solo son válidos para perfiles sin builder", perfil.Codigo)
	}
	for i := range s.Facturas {
		if s.Facturas[i].Cuf != "" {
			return fmt.Errorf("siat paquete factura %d: el XML persistido es obligatorio para conservar el CUF emitido", i+1)
		}
		if err := s.Facturas[i].validate(); err != nil {
			return fmt.Errorf("siat paquete factura %d: %w", i+1, err)
		}
	}
	return nil
}

func (s SolicitudPaqueteFactura) hasPersistedXML() bool {
	for _, factura := range s.Facturas {
		if factura.XML != "" {
			return true
		}
	}
	return false
}

// validarXMLPersistido comprueba la identidad histórica sin reserializar el XML
// ni cambiar su firma. El CUFD del sobre corresponde al envío; el del XML, a la
// emisión durante la contingencia.
func validarXMLPersistido(f SolicitudFactura, perfil *SectorProfile, paquete SolicitudPaqueteFactura) error {
	if strings.TrimSpace(f.XML) == "" || strings.TrimSpace(f.Cuf) == "" || strings.TrimSpace(f.Cufd) == "" || strings.TrimSpace(f.CodigoControl) == "" {
		return fmt.Errorf("XML, CUF, CUFD y código de control históricos son obligatorios")
	}
	if f.Nit != paquete.Nit || f.CodigoAmbiente != paquete.CodigoAmbiente || f.CodigoSistema != paquete.CodigoSistema ||
		f.Modalidad != paquete.Modalidad || f.CodigoSucursal != paquete.CodigoSucursal || f.CodigoPuntoVenta != paquete.CodigoPuntoVenta ||
		f.CodigoDocumentoSector != perfil.Codigo || f.Layout != paquete.Layout ||
		perfil.TipoDocumentoResuelto(f.CodigoTipoFactura) != perfil.TipoDocumentoResuelto(paquete.CodigoTipoFactura) {
		return fmt.Errorf("la identidad fiscal no coincide con el paquete")
	}
	var document struct {
		Cabecera struct {
			Nit                   int64  `xml:"nitEmisor"`
			Cuf                   string `xml:"cuf"`
			Cufd                  string `xml:"cufd"`
			NumeroFactura         int64  `xml:"numeroFactura"`
			CodigoDocumentoSector int    `xml:"codigoDocumentoSector"`
			CodigoSucursal        int    `xml:"codigoSucursal"`
			CodigoPuntoVenta      int    `xml:"codigoPuntoVenta"`
			FechaEmision          string `xml:"fechaEmision"`
		} `xml:"cabecera"`
	}
	if err := xml.Unmarshal([]byte(f.XML), &document); err != nil {
		return fmt.Errorf("XML persistido inválido: %w", err)
	}
	h := document.Cabecera
	if h.Cuf != f.Cuf || h.Cufd != f.Cufd || h.Nit != parseNit(f.Nit) || h.NumeroFactura != f.NumeroFactura ||
		h.CodigoDocumentoSector != f.CodigoDocumentoSector || h.CodigoSucursal != f.CodigoSucursal || h.CodigoPuntoVenta != f.CodigoPuntoVenta {
		return fmt.Errorf("la cabecera XML no coincide con la identidad fiscal persistida")
	}
	fecha, err := time.ParseInLocation("2006-01-02T15:04:05.000", h.FechaEmision, LaPaz)
	if err != nil || !fecha.Equal(f.FechaEmision.Truncate(time.Millisecond)) {
		return fmt.Errorf("fechaEmision XML no coincide con la fecha persistida")
	}
	// El CUF codifica modalidad, emisión y tipo de factura, que no aparecen como
	// campos separados en la cabecera. El SDK verifica aquí el valor existente.
	expectedCUF, err := utils.NewCUF().WithNit(h.Nit).WithFechaHora(fecha).
		WithSucursal(f.CodigoSucursal).WithModalidad(f.Modalidad).WithTipoEmision(EmisionPaqueteOffline).
		WithTipoFactura(perfil.TipoDocumentoResuelto(f.CodigoTipoFactura)).WithTipoDocumentoSector(perfil.Codigo).
		WithNumeroFactura(f.NumeroFactura).WithPuntoVenta(f.CodigoPuntoVenta).WithCodigoControl(f.CodigoControl).Generate()
	if err != nil {
		return fmt.Errorf("validar CUF persistido: %w", err)
	}
	if expectedCUF != f.Cuf {
		return fmt.Errorf("CUF persistido no corresponde a la emisión de contingencia y su contexto histórico")
	}
	return nil
}

func empaquetarXMLPersistidos(facturas []SolicitudFactura) (archivo, hash string, err error) {
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	for i, factura := range facturas {
		data := []byte(factura.XML)
		if err := writer.WriteHeader(&tar.Header{Name: fmt.Sprintf("factura_%d.xml", i+1), Mode: 0600, Size: int64(len(data))}); err != nil {
			return "", "", err
		}
		if _, err := writer.Write(data); err != nil {
			return "", "", err
		}
	}
	if err := writer.Close(); err != nil {
		return "", "", err
	}
	hash, archivo, err = utils.CompressAndHash(buffer.Bytes())
	return archivo, hash, err
}
