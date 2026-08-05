package siat

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/models"
	"gorm.io/gorm"
)

// siatFechaLayout es el formato de fecha/hora usado en fechaEnvio del SIAT.
const siatFechaLayout = "2006-01-02T15:04:05.000"

// boliviaZone es la zona horaria de Bolivia (UTC-4). El SIAT valida fechaEnvio
// contra su reloj local con una tolerancia de ±5 minutos, por lo que la hora
// debe enviarse en horario de Bolivia y no en UTC.
var boliviaZone = time.FixedZone("BOT", -4*60*60)

type EmissionService struct {
	db     *gorm.DB
	client *Client
	logger *slog.Logger
	cfg    Config
	signer *Signer
}

func NewEmissionService(db *gorm.DB, client *Client, cfg Config, logger *slog.Logger) *EmissionService {
	if logger == nil {
		logger = slog.Default()
	}
	s := &EmissionService{db: db, client: client, logger: logger, cfg: cfg}
	signer, err := NewSignerFromConfig(cfg)
	if err != nil {
		logger.Warn("emission: certificado de firma no configurado; la emisión en línea lo requerirá", "error", err.Error())
	} else {
		s.signer = signer
	}
	return s
}

// Emit genera el CUF, construye el XML conforme al XSD, lo firma (XMLDSig),
// lo comprime en GZIP y lo envía al SIAT (operación recepcionFactura).
// Actualiza la factura con el estado y el código de recepción devueltos.
func (s *EmissionService) Emit(ctx context.Context, invoiceID string) (*models.Invoice, error) {
	var inv models.Invoice
	if err := s.db.Preload("Items").Preload("PointOfSale").Preload("Company").Preload("Customer").Preload("CufdRecord").First(&inv, "id = ?", invoiceID).Error; err != nil {
		return nil, fmt.Errorf("emit: invoice not found: %w", err)
	}
	if s.client == nil {
		return nil, fmt.Errorf("emit: siat client no inicializado")
	}

	now := time.Now().UTC()

	// 1. CUFD vigente (renueva vía SIAT si expiró o falta).
	cufd, err := s.ensureCufd(ctx, &inv, now)
	if err != nil {
		return nil, err
	}
	inv.CufdId = cufd.ID

	// 2. Parámetros de emisión.
	modalidad := s.cfg.CodigoModalidad
	if modalidad <= 0 {
		modalidad = 1
	}
	codigoPuntoVenta := inv.PointOfSale.CodigoPuntoVenta
	if inv.PointOfSale.SiatCode != nil {
		codigoPuntoVenta = *inv.PointOfSale.SiatCode
	}
	nit, err := strconv.ParseInt(strings.TrimSpace(inv.Company.Nit), 10, 64)
	if err != nil {
		return nil, fmt.Errorf("emit: NIT de la empresa inválido: %w", err)
	}
	actividad := ""
	if inv.Company.CodigoActividad != nil {
		actividad = *inv.Company.CodigoActividad
	}
	if strings.TrimSpace(actividad) == "" {
		return nil, fmt.Errorf("emit: la empresa no tiene código de actividad económica (CodigoActividad)")
	}

	// 3. CUF (Código Único de Facturación).
	cuf, err := (CUFParams{
		Nit:                   nit,
		FechaHora:             inv.IssueDate.In(boliviaZone),
		CodigoSucursal:        inv.PointOfSale.CodigoSucursal,
		Modalidad:             modalidad,
		TipoEmision:           1, // Online
		TipoFactura:           1, // con derecho a crédito fiscal
		CodigoDocumentoSector: 1, // Factura de Compra Venta
		NumeroFactura:         inv.InvoiceNumber,
		CodigoPuntoVenta:      codigoPuntoVenta,
		CodigoControl:         cufd.CodigoControl,
	}).GenerarCUF()
	if err != nil {
		return nil, fmt.Errorf("emit: generar CUF: %w", err)
	}
	inv.Cuf = &cuf

	// 4. XML conforme al XSD de facturaElectronicaCompraVenta (sector 1,
	//    recibido por el servicio ServicioFacturacionCompraVenta).
	xmlPayload, err := BuildFacturaXML(InvoiceXMLParams{
		Invoice:               &inv,
		Cuf:                   cuf,
		Cufd:                  cufd.Cufd,
		CodigoControl:         cufd.CodigoControl,
		DireccionSucursal:     inv.Company.Direccion,
		Municipio:             inv.Company.Municipio,
		Telefono:              inv.Company.Telefono,
		PiePagina:             inv.Company.PiePagina,
		CodigoActividad:       actividad,
		Modalidad:             modalidad,
		TipoEmision:           1,
		CodigoDocumentoSector: 1,
		CodigoPuntoVenta:      codigoPuntoVenta,
	})
	if err != nil {
		return nil, fmt.Errorf("emit: construir XML: %w", err)
	}

	// 5. Firma XMLDSig (obligatoria en la modalidad Electrónica en Línea).
	if s.signer == nil {
		return nil, fmt.Errorf("emit: no hay certificado de firma digital configurado (SIAT_CERT_PATH o SIAT_CERT_PEM_CERT/SIAT_CERT_PEM_KEY)")
	}
	signedXML, err := s.signer.SignXML(xmlPayload)
	if err != nil {
		return nil, fmt.Errorf("emit: firmar XML: %w", err)
	}

	// 6. Comprimir (GZIP) y calcular hash (SHA256) del archivo comprimido.
	hashArchivo, archivo, err := CompressAndHash(signedXML)
	if err != nil {
		return nil, fmt.Errorf("emit: %w", err)
	}
	xmlStr := string(signedXML)
	inv.Xml = &xmlStr
	inv.XmlHash = &hashArchivo

	// 7. CUIS vigente para la solicitud.
	cuis, err := s.activeCuis(inv.PointOfSaleId, now)
	if err != nil {
		return nil, fmt.Errorf("emit: %w", err)
	}

	// 8. Enviar al SIAT (ServicioFacturacionElectronica / recepcionFactura).
	solicitud := SolicitudServicioRecepcionFactura{
		CodigoAmbiente:        inv.Company.Ambiente.CodigoAmbiente(),
		CodigoDocumentoSector: 1,
		CodigoEmision:         1, // Online
		CodigoModalidad:       modalidad,
		CodigoPuntoVenta:      codigoPuntoVenta,
		CodigoSistema:         inv.Company.CodigoSistema,
		CodigoSucursal:        inv.PointOfSale.CodigoSucursal,
		Cufd:                  cufd.Cufd,
		Cuis:                  cuis,
		Nit:                   inv.Company.Nit,
		TipoFacturaDocumento:  1,
		Archivo:               archivo,
		FechaEnvio:            now.In(boliviaZone).Format(siatFechaLayout),
		HashArchivo:           hashArchivo,
	}

	rawResp, err := s.recepcionFactura(ctx, solicitud)
	if err != nil {
		s.recordEvent(&inv, "EMIT_SEND_FAIL", "envío a SIAT falló", map[string]any{"error": err.Error()})
		return &inv, fmt.Errorf("emit: envío a SIAT: %w", err)
	}

	resp, err := parseRespuestaServicioFacturacion(rawResp)
	if err != nil {
		s.recordEvent(&inv, "EMIT_RESPONSE_PARSE_FAIL", "no se pudo interpretar la respuesta del SIAT", map[string]any{"error": err.Error(), "raw": string(rawResp)})
		return &inv, fmt.Errorf("emit: parsear respuesta: %w", err)
	}

	inv.Status = emissionStatus(resp.CodigoEstado)
	receptionCode := resp.CodigoRecepcion
	if receptionCode == "" {
		receptionCode = resp.CodigoEstado
	}
	inv.SiatReceptionCode = &receptionCode

	if err := s.db.Save(&inv).Error; err != nil {
		return &inv, fmt.Errorf("emit: guardar invoice: %w", err)
	}

	s.recordEvent(&inv, "EMIT_SEND_OK", "factura enviada al SIAT", map[string]any{
		"codigo_estado":    resp.CodigoEstado,
		"codigo_recepcion": resp.CodigoRecepcion,
		"descripcion":      resp.CodigoDescripcion,
		"transaccion":      resp.Transaccion,
		"mensajes":         resp.Mensajes,
	})

	if s.logger != nil {
		s.logger.Info("emit: factura enviada", "invoice_id", inv.ID, "cuf", cuf,
			"codigo_estado", resp.CodigoEstado, "codigo_recepcion", resp.CodigoRecepcion)
	}

	return &inv, nil
}

// ensureCufd devuelve un CUFD vigente para el punto de venta. Reutiliza el de
// la factura si sigue vigente, luego busca uno activo en BD y, si no existe,
// lo solicita al SIAT y lo persiste.
func (s *EmissionService) ensureCufd(ctx context.Context, inv *models.Invoice, now time.Time) (*models.Cufd, error) {
	if inv.CufdRecord.ID != "" && inv.CufdRecord.ValidFrom.Before(now) && inv.CufdRecord.ValidTo.After(now) {
		c := inv.CufdRecord
		return &c, nil
	}

	var active models.Cufd
	err := s.db.Where("point_of_sale_id = ? AND valid_from <= ? AND valid_to >= ? AND active = true",
		inv.PointOfSaleId, now, now).Order("created_at DESC").First(&active).Error
	if err == nil {
		return &active, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("emit: buscar CUFD activo: %w", err)
	}

	return s.requestCufd(ctx, inv, now)
}

// requestCufd solicita un nuevo CUFD al SIAT y lo persiste como activo,
// desactivando los anteriores del punto de venta.
func (s *EmissionService) requestCufd(ctx context.Context, inv *models.Invoice, now time.Time) (*models.Cufd, error) {
	cuis, err := s.activeCuis(inv.PointOfSaleId, now)
	if err != nil {
		return nil, err
	}
	codigoPuntoVenta := inv.PointOfSale.CodigoPuntoVenta
	if inv.PointOfSale.SiatCode != nil {
		codigoPuntoVenta = *inv.PointOfSale.SiatCode
	}
	modalidad := s.cfg.CodigoModalidad
	if modalidad <= 0 {
		modalidad = 1
	}

	req := SolicitudCufd{
		CodigoAmbiente:   inv.Company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    inv.Company.CodigoSistema,
		Nit:              inv.Company.Nit,
		CodigoSucursal:   inv.PointOfSale.CodigoSucursal,
		Cuis:             cuis,
		CodigoModalidad:  modalidad,
		CodigoPuntoVenta: codigoPuntoVenta,
	}

	svc := NewCufdService(s.clientFor(ServiceCodigos))
	resp, err := svc.SolicitarCUFD(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("emit: solicitar CUFD: %w", err)
	}
	if !resp.Transaccion {
		return nil, fmt.Errorf("emit: solicitar CUFD: transaccion = false (codigo %s)", resp.Codigo)
	}

	cufd := &models.Cufd{
		PointOfSaleId: inv.PointOfSaleId,
		Cufd:          resp.Codigo,
		Direccion:     resp.Direccion,
		CodigoControl: resp.CodigoControl,
		CodigoQR:      resp.CodigoQR,
		ValidFrom:     now,
		ValidTo:       resp.FechaVigencia.Time,
		Active:        true,
	}
	if err := s.db.Create(cufd).Error; err != nil {
		return nil, fmt.Errorf("emit: persistir CUFD: %w", err)
	}
	if err := s.db.Model(&models.Cufd{}).
		Where("point_of_sale_id = ? AND id <> ?", inv.PointOfSaleId, cufd.ID).
		Update("active", false).Error; err != nil {
		return nil, fmt.Errorf("emit: desactivar CUFDs anteriores: %w", err)
	}
	return cufd, nil
}

// activeCuis devuelve el CUIS vigente del punto de venta.
func (s *EmissionService) activeCuis(pointOfSaleID string, at time.Time) (string, error) {
	var cuis models.Cuis
	if err := s.db.Where("point_of_sale_id = ? AND valid_from <= ? AND valid_to >= ? AND active = true",
		pointOfSaleID, at, at).Order("created_at DESC").First(&cuis).Error; err != nil {
		return "", fmt.Errorf("CUIS vigente no encontrado para el punto de venta: %w", err)
	}
	return cuis.Cuis, nil
}

// recepcionFactura envía la solicitud a ServicioFacturacionCompraVenta.
// La factura de compra-venta (sector 1) se recibe en el servicio dedicado
// ServicioFacturacionCompraVenta; ServicioFacturacionElectronica no lo atiende
// (responde 995 "SERVICIO NO DISPONIBLE: Para la modalidad 1 y/o sector 1").
func (s *EmissionService) recepcionFactura(ctx context.Context, solicitud SolicitudServicioRecepcionFactura) ([]byte, error) {
	envelope := recepcionFacturaSOAPRequestEnvelope{
		XmlnsSo:  soapEnvelopeNamespace,
		XmlnsNs:  siatNamespace,
		XmlnsXsd: "http://www.w3.org/2001/XMLSchema",
		XmlnsXsi: "http://www.w3.org/2001/XMLSchema-instance",
		Body: recepcionFacturaSOAPBody{
			Request: recepcionFacturaOperation{Solicitud: solicitud},
		},
	}
	payload, err := xml.Marshal(envelope)
	if err != nil {
		return nil, fmt.Errorf("siat recepcion factura: marshal request: %w", err)
	}
	return s.clientFor(ServiceFacturacionCompraVenta).DoRaw(ctx, "recepcionFactura", payload)
}

// clientFor devuelve un clon del cliente apuntando al endpoint del servicio indicado.
func (s *EmissionService) clientFor(service Service) *Client {
	return s.client.CloneWithEndpoint(s.cfg.ServiceEndpoint(service))
}

// emissionStatus mapea el codigoEstado del SIAT al estado local de la factura.
func emissionStatus(codigoEstado string) models.InvoiceStatus {
	switch codigoEstado {
	case CodigoEstadoValidada, CodigoEstadoProcesada:
		return models.StatusAccepted
	case CodigoEstadoRechazada:
		return models.StatusRejected
	case CodigoEstadoObservada:
		return models.StatusObserved
	case CodigoEstadoPendiente:
		return models.StatusPending
	default:
		return models.StatusSent
	}
}

// recordEvent persiste un evento de auditoría de la factura.
func (s *EmissionService) recordEvent(inv *models.Invoice, eventType, message string, payload any) {
	jsonPayload, _ := json.Marshal(payload)
	s.db.Create(&models.InvoiceEvent{
		InvoiceId: inv.ID,
		Type:      eventType,
		Message:   message,
		Payload:   jsonPayload,
		CreatedAt: time.Now().UTC(),
	})
}
