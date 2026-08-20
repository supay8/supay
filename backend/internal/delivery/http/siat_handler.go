package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/models"
	"github.com/brandsrx/supay/internal/pdf"
	"github.com/brandsrx/supay/internal/siat"
	goSiat "github.com/ron86i/go-siat/v2"
	"github.com/go-chi/chi/v5"
)

type SiatHandler struct {
	companyRepo     domain.CompanyRepository
	pointOfSaleRepo domain.PointOfSaleRepository
	cufdRepo        domain.CufdRepository
	tipoPVRepo      domain.TipoPuntoVentaRepository
	catalogRepo     domain.CatalogRepository
	contingencyRepo domain.ContingencyEventRepository
	sentPackageRepo domain.SentPackageRepository
	siatService     *siat.Service
	pdfService      *pdf.Service
	modalidad       int
}

func NewSiatHandler(
	companyRepo domain.CompanyRepository,
	pointOfSaleRepo domain.PointOfSaleRepository,
	cufdRepo domain.CufdRepository,
	tipoPVRepo domain.TipoPuntoVentaRepository,
	catalogRepo domain.CatalogRepository,
	contingencyRepo domain.ContingencyEventRepository,
	sentPackageRepo domain.SentPackageRepository,
	siatService *siat.Service,
	pdfService *pdf.Service,
	modalidad int,
) *SiatHandler {
	return &SiatHandler{
		companyRepo:     companyRepo,
		pointOfSaleRepo: pointOfSaleRepo,
		cufdRepo:        cufdRepo,
		tipoPVRepo:      tipoPVRepo,
		catalogRepo:     catalogRepo,
		contingencyRepo: contingencyRepo,
		sentPackageRepo: sentPackageRepo,
		siatService:     siatService,
		pdfService:      pdfService,
		modalidad:       modalidad,
	}
}

type siatCuisResponse struct {
	Company     *domain.Company     `json:"company"`
	PointOfSale *domain.PointOfSale `json:"point_of_sale"`
	Response    *siat.RespuestaCuis `json:"response"`
}

type siatCufdResponse struct {
	Company     *domain.Company     `json:"company"`
	PointOfSale *domain.PointOfSale `json:"point_of_sale"`
	Response    *siat.RespuestaCufd `json:"response"`
}

type siatEventoSignificativoRequest struct {
	CodigoMotivoEvento    int    `json:"codigoMotivoEvento"`
	Descripcion           string `json:"descripcion"`
	CufdEvento            string `json:"cufdEvento"`
	FechaHoraInicioEvento string `json:"fechaHoraInicioEvento"`
	FechaHoraFinEvento    string `json:"fechaHoraFinEvento"`
}

type siatEventoSignificativoResponse struct {
	Company     *domain.Company                    `json:"company"`
	PointOfSale *domain.PointOfSale                `json:"point_of_sale"`
	Response    *siat.ResultadoEventoSignificativo `json:"response"`
}

func (h *SiatHandler) SolicitarCUIS(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	codigoPuntoVenta := pointOfSale.CodigoPuntoVenta
	if pointOfSale.SiatCode != nil {
		codigoPuntoVenta = *pointOfSale.SiatCode
	}
	modalidad := h.modalidad
	if modalidad <= 0 {
		modalidad = 1
	}

	req := siat.SolicitudCuis{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		CodigoModalidad:  modalidad,
		CodigoPuntoVenta: codigoPuntoVenta,
	}
	if pointOfSale.Cuis != nil && *pointOfSale.Cuis != "" {
		req.Cuis = pointOfSale.Cuis
	}

	resp, err := h.siatService.SolicitarCUIS(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	now := time.Now().In(siat.LaPaz)
	updatedCuis := resp.Codigo
	pointOfSale.Cuis = &updatedCuis
	pointOfSale.CuisCreatedAt = &now
	if err := h.pointOfSaleRepo.Update(pointOfSale); err != nil {
		http.Error(w, "No se pudo persistir el CUIS en el punto de venta", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatCuisResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    resp,
	})
}

func (h *SiatHandler) SolicitarCUFD(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		http.Error(w, "El punto de venta no tiene CUIS activo", http.StatusConflict)
		return
	}

	codigoPuntoVenta := pointOfSale.CodigoPuntoVenta
	if pointOfSale.SiatCode != nil {
		codigoPuntoVenta = *pointOfSale.SiatCode
	}
	modalidad := h.modalidad
	if modalidad <= 0 {
		modalidad = 1
	}

	req := siat.SolicitudCufd{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		Cuis:             *pointOfSale.Cuis,
		CodigoModalidad:  modalidad,
		CodigoPuntoVenta: codigoPuntoVenta,
	}

	resp, err := h.siatService.SolicitarCUFD(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	now := time.Now().In(siat.LaPaz)
	cufd := &domain.Cufd{
		PointOfSaleID: pointOfSale.ID,
		Cufd:          resp.Codigo,
		ControlCode:   resp.CodigoControl,
		Direccion:     resp.Direccion,
		ValidFrom:     now,
		ValidTo:       resp.FechaVigencia.Time,
		Active:        true,
	}
	if err := h.cufdRepo.Create(cufd); err != nil {
		http.Error(w, "No se pudo persistir el CUFD", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatCufdResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    resp,
	})
}

// RegistrarEventoSignificativo registra una contingencia (p.ej. corte de
// internet, codigoMotivoEvento=1) ante el SIAT (registroEventoSignificativo)
// para la empresa y punto de venta indicados. El CUFD vigente se usa como
// cufdEvento salvo que el body lo sobrescriba (p.ej. el CUFD vencido durante la
// contingencia).
func (h *SiatHandler) RegistrarEventoSignificativo(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		http.Error(w, "El punto de venta no tiene CUIS activo", http.StatusConflict)
		return
	}

	cufd, err := h.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		http.Error(w, "El punto de venta no tiene CUFD vigente", http.StatusConflict)
		return
	}

	var body siatEventoSignificativoRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
			return
		}
	}

	codigoMotivo := body.CodigoMotivoEvento
	if codigoMotivo <= 0 {
		codigoMotivo = siat.MotivoCorteInternet
	}
	descripcion := strings.TrimSpace(body.Descripcion)
	if descripcion == "" {
		descripcion = "Corte del servicio de internet"
	}
	cufdEvento := strings.TrimSpace(body.CufdEvento)
	if cufdEvento == "" {
		cufdEvento = cufd.Cufd
	}

	inicio, err := parseFechaSiat(body.FechaHoraInicioEvento)
	if err != nil {
		http.Error(w, "fechaHoraInicioEvento inválida (use YYYY-MM-DDTHH:mm:ss.SSS)", http.StatusBadRequest)
		return
	}
	fin, err := parseFechaSiat(body.FechaHoraFinEvento)
	if err != nil {
		http.Error(w, "fechaHoraFinEvento inválida (use YYYY-MM-DDTHH:mm:ss.SSS)", http.StatusBadRequest)
		return
	}

	codigoPuntoVenta := pointOfSale.CodigoPuntoVenta
	if pointOfSale.SiatCode != nil {
		codigoPuntoVenta = *pointOfSale.SiatCode
	}
	modalidad := h.modalidad
	if modalidad <= 0 {
		modalidad = 1
	}

	req := siat.SolicitudEventoSignificativo{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         company.CodigoSistema,
		Nit:                   company.Nit,
		CodigoSucursal:        pointOfSale.CodigoSucursal,
		CodigoPuntoVenta:      codigoPuntoVenta,
		Cuis:                  *pointOfSale.Cuis,
		Cufd:                  cufd.Cufd,
		CufdEvento:            cufdEvento,
		CodigoMotivoEvento:    codigoMotivo,
		Descripcion:           descripcion,
		FechaHoraInicioEvento: inicio,
		FechaHoraFinEvento:    fin,
	}
	result, err := h.siatService.RegistrarEventoSignificativo(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	// Se persiste el evento registrado (con su codigoRecepcion del SIAT) para
	// que el envío de paquetes pueda resolver automáticamente el codigoEvento.
	if h.contingencyRepo != nil && result != nil && result.Transaccion && result.CodigoRecepcion != "" {
		ev := &domain.ContingencyEvent{
			PointOfSaleID: pointOfSale.ID,
			Reason:        motivoEventoAReason(codigoMotivo),
			Description:   &descripcion,
			StartDate:     inicio,
			EndDate:       &fin,
			SiatEventCode: &result.CodigoRecepcion,
			IsSynced:      true,
		}
		if err := h.contingencyRepo.Create(ev); err != nil {
			// Log el error pero no falla la respuesta: el evento SIAT ya se registró
			// exitosamente; la persistencia local es complementaria. Un error aquí
			// implica que la resolución automática de codigoEvento no funcionará
			// para envíos posteriores de paquetes.
			fmt.Printf("WARNING: no se pudo persistir evento de contingencia (SIAT code=%s): %v\n", result.CodigoRecepcion, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatEventoSignificativoResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    result,
	})
}

// motivoEventoAReason traduce el codigoMotivoEvento del catálogo del SIAT a la
// razón de contingencia local (enum de models.ContingencyReason).
func motivoEventoAReason(motivo int) string {
	if motivo == siat.MotivoCorteInternet {
		return string(models.ReasonFallaInternet)
	}
	return string(models.ReasonOtro)
}

type siatPaqueteRequest struct {
	CodigoEvento  int                     `json:"codigoEvento"`
	Descripcion   string                  `json:"descripcion"`
	CodigoEmision int                     `json:"codigoEmision"`
	Facturas      []siat.SolicitudFactura `json:"facturas"`
}

type siatMasivaRequest struct {
	CodigoEmision int                     `json:"codigoEmision"`
	Facturas      []siat.SolicitudFactura `json:"facturas"`
}

type siatComprasRequest struct {
	Descripcion      string    `json:"descripcion"`
	TipoCompra       int       `json:"tipoCompra"`
	Archivo          string    `json:"archivo"`
	HashArchivo      string    `json:"hashArchivo"`
	CantidadFacturas int       `json:"cantidadFacturas"`
	Gestion          int       `json:"gestion"`
	Periodo          int       `json:"periodo"`
	FechaEnvio       time.Time `json:"fechaEnvio"`
}

type siatPaqueteValidacionRequest struct {
	CodigoRecepcion string `json:"codigoRecepcion"`
	CodigoEmision   int    `json:"codigoEmision"`
	CodigoDocSector int    `json:"codigoDocumentoSector"`
	CodigoTipoFact  int    `json:"codigoTipoFactura"`
}

type siatFirmaRequest struct {
	Xml string `json:"xml"`
}

type siatFirmaResponse struct {
	Company     *domain.Company      `json:"company"`
	PointOfSale *domain.PointOfSale  `json:"point_of_sale"`
	Response    *siat.ResultadoFirma `json:"response"`
}

type siatPaqueteResponse struct {
	Company     *domain.Company        `json:"company"`
	PointOfSale *domain.PointOfSale    `json:"point_of_sale"`
	Response    *siat.ResultadoPaquete `json:"response"`
}

type siatComprasResponse struct {
	Company     *domain.Company        `json:"company"`
	PointOfSale *domain.PointOfSale    `json:"point_of_sale"`
	Response    *siat.ResultadoCompras `json:"response"`
}

// buildSolicitudPaquete reúne la identidad del contribuyente (empresa + punto
// de venta + CUIS/CUFD vigente) y la mezcla con los datos específicos del
// paquete enviados en el body (codigoEvento, descripcion, codigoEmision y
// facturas).
func (h *SiatHandler) buildSolicitudPaquete(r *http.Request, body siatPaqueteRequest) (*siat.SolicitudPaqueteFactura, *domain.Company, *domain.PointOfSale, error) {
	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		return nil, nil, nil, err
	}

	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		return nil, nil, nil, &conflictError{message: "El punto de venta no tiene CUIS activo"}
	}

	cufd, err := h.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		return nil, nil, nil, &conflictError{message: "El punto de venta no tiene CUFD vigente"}
	}

	codigoPuntoVenta := pointOfSale.CodigoPuntoVenta
	if pointOfSale.SiatCode != nil {
		codigoPuntoVenta = *pointOfSale.SiatCode
	}
	modalidad := h.modalidad
	if modalidad <= 0 {
		modalidad = siat.ModalidadElectronica
	}
	var facturasProcesadas []siat.SolicitudFactura
	loc, _ := time.LoadLocation("America/La_Paz")
	if loc == nil {
		loc = time.FixedZone("BOT", -4*60*60)
	}
	for _, f := range body.Facturas {
		f.CodigoAmbiente = company.Ambiente.CodigoAmbiente()
		f.CodigoSistema = company.CodigoSistema
		f.Nit = company.Nit
		f.Modalidad = modalidad
		f.CodigoSucursal = pointOfSale.CodigoSucursal
		f.CodigoPuntoVenta = codigoPuntoVenta
		f.Cuis = *pointOfSale.Cuis
		f.Cufd = cufd.Cufd
		f.CodigoControl = cufd.ControlCode

		if f.FechaEmision.IsZero() {
			f.FechaEmision = time.Now().In(loc)
		}
		facturasProcesadas = append(facturasProcesadas, f)
	}

	// Resolver documento-sector desde la primera factura del paquete o desde la
	// empresa (catálogo actividadesDocumentoSector).  El hardcode "1" provocaba
	// que facturas del sector educativo (11) se enviaran con sector incorrecto.
	codigoDocSector := 0
	codigoTipoFact := 0
	if len(facturasProcesadas) > 0 {
		codigoDocSector = facturasProcesadas[0].CodigoDocumentoSector
		codigoTipoFact = facturasProcesadas[0].CodigoTipoFactura
	}
	if codigoDocSector <= 0 {
		codigoDocSector = h.resolveDocumentoSector(company)
	}
	if codigoTipoFact <= 0 {
		codigoTipoFact = 1
	}

	req := &siat.SolicitudPaqueteFactura{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         company.CodigoSistema,
		Nit:                   company.Nit,
		Modalidad:             modalidad,
		CodigoSucursal:        pointOfSale.CodigoSucursal,
		CodigoPuntoVenta:      codigoPuntoVenta,
		Cuis:                  *pointOfSale.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoControl:         cufd.ControlCode,
		CodigoDocumentoSector: codigoDocSector,
		CodigoTipoFactura:     codigoTipoFact,
		CodigoEmision:         body.CodigoEmision,
		CodigoEvento:          int64(body.CodigoEvento),
		Descripcion:           body.Descripcion,
		Facturas:              facturasProcesadas,
	}
	return req, company, pointOfSale, nil
}

// EnviarPaquete envía al SIAT un paquete de facturas (recepcionPaqueteFactura)
// para la empresa y punto de venta indicados, en el contexto de un evento
// significativo registrado (codigoEvento), p.ej. una contingencia por corte del
// servicio de internet.
func (h *SiatHandler) EnviarPaquete(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	var body siatPaqueteRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
			return
		}
	}
	if len(body.Facturas) == 0 {
		http.Error(w, "El paquete debe contener al menos una factura en el campo facturas", http.StatusBadRequest)
		return
	}
	req, company, pointOfSale, err := h.buildSolicitudPaquete(r, body)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	// codigoEvento debe ser el codigoRecepcion del último evento significativo
	// registrado en el SIAT (registroEventoSignificativo); si no se envía, se
	// resuelve automáticamente desde el último evento persistido del punto de
	// venta. Un código inventado hace que el SIAT lo rechace (código 942).
	if req.CodigoEvento <= 0 && h.contingencyRepo != nil {
		if ev, evErr := h.contingencyRepo.GetLatestByPointOfSale(pointOfSale.ID); evErr == nil && ev.SiatEventCode != nil {
			if code, cErr := strconv.ParseInt(*ev.SiatEventCode, 10, 64); cErr == nil && code > 0 {
				req.CodigoEvento = code
			}
		}
	}
	if req.CodigoEvento <= 0 {
		http.Error(w, "codigoEvento es obligatorio: registre primero un evento significativo (POST /evento-significativo/{companyId}/{pointOfSaleId}) y use el codigoRecepcion de la respuesta, o envíelo vacío para tomar el último evento registrado", http.StatusBadRequest)
		return
	}

	result, err := h.siatService.EnviarPaqueteFactura(r.Context(), *req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if h.sentPackageRepo != nil && result != nil && result.CodigoRecepcion != "" {
		pkg := &domain.SentPackage{
			CompanyId:           company.ID,
			PointOfSaleId:       pointOfSale.ID,
			CodigoRecepcion:    result.CodigoRecepcion,
			Type:               domain.PackageTypePaquete,
			CodigoDocumentoSector: req.CodigoDocumentoSector,
			CodigoEmision:      int(req.CodigoEmision),
			CantidadFacturas:   len(req.Facturas),
			Status:             domain.PackageStatusPending,
		}
		_ = h.sentPackageRepo.Create(pkg)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatPaqueteResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    result,
	})
}

// ValidarPaquete consulta al SIAT la validación de un paquete ya enviado
// (validacionRecepcionPaqueteFactura) usando el codigoRecepcion devuelto por
// el envío.
func (h *SiatHandler) ValidarPaquete(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	var body siatPaqueteValidacionRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
			return
		}
	}
	if strings.TrimSpace(body.CodigoRecepcion) == "" {
		http.Error(w, "codigoRecepcion es obligatorio (código devuelto por el envío del paquete)", http.StatusBadRequest)
		return
	}

	req, company, pointOfSale, err := h.buildSolicitudPaquete(r, siatPaqueteRequest{
		CodigoEmision: body.CodigoEmision,
	})
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}
	if body.CodigoDocSector > 0 {
		req.CodigoDocumentoSector = body.CodigoDocSector
	}
	if body.CodigoTipoFact > 0 {
		req.CodigoTipoFactura = body.CodigoTipoFact
	}

	result, err := h.siatService.ValidarPaqueteFactura(r.Context(), *req, body.CodigoRecepcion)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatPaqueteResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    result,
	})
}

// buildSolicitudMasiva reúne la identidad del contribuyente (empresa + punto de
// venta + CUIS/CUFD vigente) y la mezcla con los datos específicos del lote
// enviados en el body (codigoEmision y facturas).
func (h *SiatHandler) buildSolicitudMasiva(r *http.Request, body siatMasivaRequest) (*siat.SolicitudMasivaFactura, *domain.Company, *domain.PointOfSale, error) {
	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		return nil, nil, nil, err
	}

	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		return nil, nil, nil, &conflictError{message: "El punto de venta no tiene CUIS activo"}
	}

	cufd, err := h.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		return nil, nil, nil, &conflictError{message: "El punto de venta no tiene CUFD vigente"}
	}

	codigoPuntoVenta := pointOfSale.CodigoPuntoVenta
	if pointOfSale.SiatCode != nil {
		codigoPuntoVenta = *pointOfSale.SiatCode
	}
	modalidad := h.modalidad
	if modalidad <= 0 {
		modalidad = siat.ModalidadElectronica
	}

	// Resolver documento-sector desde la primera factura del lote o desde la
	// empresa (catálogo actividadesDocumentoSector).
	codigoDocSector := 0
	codigoTipoFact := 0
	if len(body.Facturas) > 0 {
		codigoDocSector = body.Facturas[0].CodigoDocumentoSector
		codigoTipoFact = body.Facturas[0].CodigoTipoFactura
	}
	if codigoDocSector <= 0 {
		codigoDocSector = h.resolveDocumentoSector(company)
	}
	if codigoTipoFact <= 0 {
		codigoTipoFact = 1
	}

	req := &siat.SolicitudMasivaFactura{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         company.CodigoSistema,
		Nit:                   company.Nit,
		Modalidad:             modalidad,
		CodigoSucursal:        pointOfSale.CodigoSucursal,
		CodigoPuntoVenta:      codigoPuntoVenta,
		Cuis:                  *pointOfSale.Cuis,
		Cufd:                  cufd.Cufd,
		CodigoControl:         cufd.ControlCode,
		CodigoDocumentoSector: codigoDocSector,
		CodigoTipoFactura:     codigoTipoFact,
		CodigoEmision:         body.CodigoEmision,
		Facturas:              body.Facturas,
	}

	return req, company, pointOfSale, nil
}

// EnviarMasiva envía al SIAT un lote de facturas por emisión masiva
// (recepcionMasivaFactura, codigoEmision = 3) para la empresa y punto de venta
// indicados.
func (h *SiatHandler) EnviarMasiva(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	var body siatMasivaRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
			return
		}
	}
	if len(body.Facturas) == 0 {
		http.Error(w, "El lote debe contener al menos una factura en el campo facturas", http.StatusBadRequest)
		return
	}

	req, company, pointOfSale, err := h.buildSolicitudMasiva(r, body)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}
	result, err := h.siatService.EnviarMasivaFacturas(r.Context(), *req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	if h.sentPackageRepo != nil && result != nil && result.CodigoRecepcion != "" {
		pkg := &domain.SentPackage{
			CompanyId:           company.ID,
			PointOfSaleId:       pointOfSale.ID,
			CodigoRecepcion:    result.CodigoRecepcion,
			Type:               domain.PackageTypeMasiva,
			CodigoDocumentoSector: req.CodigoDocumentoSector,
			CodigoEmision:      req.CodigoEmision,
			CantidadFacturas:   len(req.Facturas),
			Status:             domain.PackageStatusPending,
		}
		_ = h.sentPackageRepo.Create(pkg)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatPaqueteResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    result,
	})
}

// ValidarMasiva consulta al SIAT la validación de un lote ya enviado
// (validacionRecepcionMasivaFactura) usando el codigoRecepcion devuelto por el
// envío.
func (h *SiatHandler) ValidarMasiva(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	var body siatPaqueteValidacionRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
			return
		}
	}
	if strings.TrimSpace(body.CodigoRecepcion) == "" {
		http.Error(w, "codigoRecepcion es obligatorio (código devuelto por el envío del lote)", http.StatusBadRequest)
		return
	}

	req, company, pointOfSale, err := h.buildSolicitudMasiva(r, siatMasivaRequest{
		CodigoEmision: body.CodigoEmision,
	})
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}
	if body.CodigoDocSector > 0 {
		req.CodigoDocumentoSector = body.CodigoDocSector
	}
	if body.CodigoTipoFact > 0 {
		req.CodigoTipoFactura = body.CodigoTipoFact
	}

	result, err := h.siatService.ValidarMasivaFacturas(r.Context(), *req, body.CodigoRecepcion)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatPaqueteResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    result,
	})
}

// EnviarCompras registra en el SIAT un paquete de facturas de compras
// (recepcionPaqueteCompras, Etapa XI). codigoPuntoVenta no aplica en el servicio
// de compras; codigoSucursal se toma del punto de venta y la identidad
// (ambiente, sistema, NIT) de la empresa.
func (h *SiatHandler) EnviarCompras(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	var body siatComprasRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
			return
		}
	}
	if strings.TrimSpace(body.Archivo) == "" || strings.TrimSpace(body.HashArchivo) == "" {
		http.Error(w, "archivo y hashArchivo son obligatorios (Base64 del TAR.GZ y su SHA-256)", http.StatusBadRequest)
		return
	}

	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		http.Error(w, "El punto de venta no tiene CUIS activo", http.StatusConflict)
		return
	}

	cufd, err := h.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		http.Error(w, "El punto de venta no tiene CUFD vigente", http.StatusConflict)
		return
	}

	req := siat.SolicitudCompras{
		Descripcion:      body.Descripcion,
		TipoCompra:       body.TipoCompra,
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		CodigoPuntoVenta: 0,
		Cuis:             *pointOfSale.Cuis,
		Cufd:             cufd.Cufd,
		Archivo:          body.Archivo,
		HashArchivo:      body.HashArchivo,
		CantidadFacturas: body.CantidadFacturas,
		Gestion:          body.Gestion,
		Periodo:          body.Periodo,
		FechaEnvio:       body.FechaEnvio,
	}

	result, err := h.siatService.EnviarCompras(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if h.sentPackageRepo != nil && result != nil && result.CodigoRecepcion != "" {
		pkg := &domain.SentPackage{
			CompanyId:           company.ID,
			PointOfSaleId:       pointOfSale.ID,
			CodigoRecepcion:    result.CodigoRecepcion,
			Type:               domain.PackageTypeCompras,
			CodigoDocumentoSector: 19,
			CantidadFacturas:   body.CantidadFacturas,
			Status:             domain.PackageStatusPending,
		}
		_ = h.sentPackageRepo.Create(pkg)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatComprasResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    result,
	})
}

// FirmarFactura firma digitalmente el XML de una factura con el certificado de
// la empresa (P12 o PEM) usando el SDK go-siat (Etapa VIII - Firma Digital) y
// devuelve el XML firmado junto con el archivo gzip+Base64 y su hash SHA-256.
func (h *SiatHandler) FirmarFactura(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	var body siatFirmaRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
			return
		}
	}
	if strings.TrimSpace(body.Xml) == "" {
		http.Error(w, "xml es obligatorio (la cadena del XML de la factura a firmar)", http.StatusBadRequest)
		return
	}

	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	result, err := h.siatService.FirmarFacturaXML(r.Context(), body.Xml)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(siatFirmaResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    result,
	})
}

// parseFechaSiat parsea una fecha/hora del SIAT en formato UTC extendido sin
// zona horaria (YYYY-MM-DDTHH:mm:ss.SSS). Si el valor está vacío usa la fecha
// y hora actual en UTC.
func parseFechaSiat(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().In(siat.LaPaz), nil
	}
	parsed, err := time.ParseInLocation("2006-01-02T15:04:05.000", value, siat.LaPaz)
	if err != nil {
		return time.Time{}, err
	}
	return parsed, nil
}

type siatSincronizacionOpResult struct {
	Operation   string `json:"operation"`
	Transaccion bool   `json:"transaccion"`
	Codigos     int    `json:"codigos"`
	FechaHora   string `json:"fechaHora,omitempty"`
}

type siatSincronizacionOpError struct {
	Operation string `json:"operation"`
	Error     string `json:"error"`
}

type siatSincronizacionResponse struct {
	Company     *domain.Company              `json:"company"`
	PointOfSale *domain.PointOfSale          `json:"point_of_sale"`
	Operations  []siatSincronizacionOpResult `json:"operations,omitempty"`
	Errors      []siatSincronizacionOpError  `json:"errors,omitempty"`
}

// Sincronizar baja catálogos del SIAT para la empresa y punto de venta indicados.
// Sin parámetro ?operation= sincroniza todas las operaciones del SDK.
func (h *SiatHandler) Sincronizar(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		http.Error(w, "El punto de venta no tiene CUIS activo", http.StatusConflict)
		return
	}

	// El código de punto de venta que usa el CUIS debe ser el mismo en todas
	// las operaciones (preferir el código registrado ante SIAT).
	req := siat.SolicitudSincronizacion{
		CodigoAmbiente:   company.Ambiente.CodigoAmbiente(),
		CodigoSistema:    company.CodigoSistema,
		Nit:              company.Nit,
		CodigoSucursal:   pointOfSale.CodigoSucursal,
		CodigoPuntoVenta: pointOfSale.CodigoPuntoVenta,
		Cuis:             *pointOfSale.Cuis,
	}
	opRaw := r.URL.Query().Get("operation")
	if opRaw != "" {
		op, ok := siat.ParseSincronizacionOp(opRaw)
		if !ok {
			http.Error(w, "Operación de sincronización desconocida: "+opRaw, http.StatusBadRequest)
			return
		}
		result, err := h.siatService.Sincronizar(r.Context(), req, op)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadGateway)
			return
		}
		if perr := h.persistSincronizacion(company.ID, op, result); perr != nil {
			http.Error(w, "No se pudo persistir el catálogo: "+perr.Error(), http.StatusInternalServerError)
			return
		}
		h.respondSincronizacion(w, company, pointOfSale, []siatSincronizacionOpResult{toSincronizacionOpResult(op, result)}, nil)
		return
	}

	var results []siatSincronizacionOpResult
	var errors []siatSincronizacionOpError
	for _, op := range siat.SincronizacionOperations {
		result, err := h.siatService.Sincronizar(r.Context(), req, op)
		if err != nil {
			errors = append(errors, siatSincronizacionOpError{Operation: string(op), Error: err.Error()})
			continue
		}
		if perr := h.persistSincronizacion(company.ID, op, result); perr != nil {
			errors = append(errors, siatSincronizacionOpError{Operation: string(op), Error: "persistencia: " + perr.Error()})
		}
		results = append(results, toSincronizacionOpResult(op, result))
	}

	h.respondSincronizacion(w, company, pointOfSale, results, errors)
}

func toSincronizacionOpResult(op siat.SincronizacionOp, res *siat.RespuestaSincronizacion) siatSincronizacionOpResult {
	out := siatSincronizacionOpResult{
		Operation:   string(op),
		Transaccion: res.Transaccion,
		Codigos:     len(res.Codigos),
	}
	if !res.FechaHora.IsZero() {
		out.FechaHora = res.FechaHora.Format(time.RFC3339Nano)
	}
	return out
}

func (h *SiatHandler) respondSincronizacion(w http.ResponseWriter, company *domain.Company, pointOfSale *domain.PointOfSale, results []siatSincronizacionOpResult, errors []siatSincronizacionOpError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatSincronizacionResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Operations:  results,
		Errors:      errors,
	})
}

// persistSincronizacion guarda el catálogo sincronizado: tipos de punto de venta
// en tipo_punto_ventas y el resto en catalogs. FechaHora y VerificarComunicacion
// no son catálogos y no se persisten.
func (h *SiatHandler) persistSincronizacion(companyID string, op siat.SincronizacionOp, res *siat.RespuestaSincronizacion) error {
	if res == nil || !res.Transaccion {
		return nil
	}
	now := time.Now().In(siat.LaPaz)

	if op == siat.OpTipoPuntoVenta {
		tipos := make([]domain.TipoPuntoVenta, 0, len(res.Codigos))
		for _, c := range res.Codigos {
			tipos = append(tipos, domain.TipoPuntoVenta{
				CodigoClasificador: c.CodigoClasificador,
				Descripcion:        c.Descripcion,
			})
		}
		return h.tipoPVRepo.Replace(companyID, tipos, now)
	}

	switch op {
	case siat.OpFechaHora, siat.OpVerificarComunicacion:
		return nil
	}

	items := make([]domain.CatalogItem, 0, len(res.Codigos))
	for _, c := range res.Codigos {
		items = append(items, domain.CatalogItem{
			Codigo:      c.CodigoClasificador,
			Descripcion: c.Descripcion,
			Tipo:        string(op),
		})
	}
	return h.catalogRepo.Replace(companyID, string(op), items, now)
}

// DownloadPDF genera y descarga el PDF de la factura indicada.
func (h *SiatHandler) DownloadPDF(w http.ResponseWriter, r *http.Request) {
	if h.pdfService == nil {
		http.Error(w, "Servicio de PDF no inicializado", http.StatusServiceUnavailable)
		return
	}
	invoiceID := chi.URLParam(r, "invoiceId")
	if invoiceID == "" {
		http.Error(w, "invoiceId es obligatorio", http.StatusBadRequest)
		return
	}

	data, err := h.pdfService.GenerateInvoicePDF(invoiceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"factura-%s.pdf\"", invoiceID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func (h *SiatHandler) loadCompanyAndPointOfSale(r *http.Request) (*domain.Company, *domain.PointOfSale, error) {
	companyID := chi.URLParam(r, "companyId")
	pointOfSaleID := chi.URLParam(r, "pointOfSaleId")
	if companyID == "" || pointOfSaleID == "" {
		return nil, nil, &badRequestError{message: "companyId y pointOfSaleId son obligatorios"}
	}

	company, err := h.companyRepo.GetByID(companyID)
	if err != nil {
		return nil, nil, &notFoundError{message: "Empresa no encontrada"}
	}

	pointOfSale, err := h.pointOfSaleRepo.GetByID(pointOfSaleID)
	if err != nil {
		return nil, nil, &notFoundError{message: "Punto de venta no encontrado"}
	}

	if pointOfSale.CompanyId != company.ID {
		return nil, nil, &badRequestError{message: "El punto de venta no pertenece a la empresa indicada"}
	}

	return company, pointOfSale, nil
}

type badRequestError struct {
	message string
}

func (e *badRequestError) Error() string { return e.message }

type notFoundError struct {
	message string
}

func (e *notFoundError) Error() string { return e.message }

type conflictError struct {
	message string
}

func (e *conflictError) Error() string { return e.message }

// resolveDocumentoSector determina el documento-sector del SIAT para la
// actividad económica de la empresa consultando el catálogo sincronizado
// actividadesDocumentoSector.  Prefiere la factura de compraventa (FCV); si la
// actividad solo está asociada a sectores educativos (FSEDU), usa ese sector.
func (h *SiatHandler) resolveDocumentoSector(company *domain.Company) int {
	if h.catalogRepo == nil || company.CodigoActividad == nil {
		return siat.SectorCompraVenta
	}
	actividad := strings.TrimSpace(*company.CodigoActividad)
	if actividad == "" {
		return siat.SectorCompraVenta
	}
	items, err := h.catalogRepo.List(company.ID, "actividadesDocumentoSector")
	if err != nil || len(items) == 0 {
		return siat.SectorCompraVenta
	}
	found := 0
	for _, item := range items {
		fields := strings.Split(item.Descripcion, "|")
		if len(fields) < 2 || strings.TrimSpace(fields[0]) != actividad {
			continue
		}
		tipo := strings.TrimSpace(fields[1])
		if tipo == "FCV" {
			return item.Codigo
		}
		if tipo == "FSEDU" {
			found = item.Codigo
		}
	}
	if found > 0 {
		return found
	}
	return siat.SectorCompraVenta
}

func errToStatus(err error) int {
	var br *badRequestError
	var nf *notFoundError
	var cf *conflictError
	var siatErr *goSiat.SiatError
	switch {
	case errors.As(err, &br):
		return http.StatusBadRequest
	case errors.As(err, &nf):
		return http.StatusNotFound
	case errors.As(err, &cf):
		return http.StatusConflict
	case errors.As(err, &siatErr):
		if goSiat.IsNetworkError(err) || goSiat.IsRetryable(err) {
			return http.StatusServiceUnavailable
		}
		return http.StatusBadGateway
	default:
		return http.StatusInternalServerError
	}
}

// --- Documentos de Ajuste (Notas de Crédito/Débito) ---

type siatDocumentoAjusteRequest struct {
	CodigoAmbiente        int                    `json:"codigoAmbiente"`
	CodigoSistema         string                 `json:"codigoSistema"`
	Nit                   string                 `json:"nit"`
	Modalidad             int                    `json:"modalidad"`
	NumeroFactura         int64                  `json:"numeroFactura"`
	CodigoSucursal        int                    `json:"codigoSucursal"`
	CodigoPuntoVenta      int                    `json:"codigoPuntoVenta"`
	Cuis                  string                 `json:"cuis"`
	Cufd                  string                 `json:"cufd"`
	CodigoControl         string                 `json:"codigoControl"`
	CufFacturaOriginal    string                 `json:"cufFacturaOriginal"`
	CodigoDocumentoSector int                    `json:"codigoDocumentoSector"`
	CodigoTipoFactura     int                    `json:"codigoTipoFactura"`
	TipoNota              int                    `json:"tipoNota"`
	Motivo                string                 `json:"motivo"`
	CodigoMetodoPago      int                    `json:"codigoMetodoPago"`
	CodigoMoneda          int                    `json:"codigoMoneda"`
	TipoCambio            float64                `json:"tipoCambio"`
	MontoTotal            float64                `json:"montoTotal"`
	Leyenda               string                 `json:"leyenda"`
	Cliente               siat.ClienteFactura    `json:"cliente"`
	Items                 []siat.ItemFactura     `json:"items"`
}

type siatDocumentoAjusteResponse struct {
	Company     *domain.Company                   `json:"company"`
	PointOfSale *domain.PointOfSale               `json:"point_of_sale"`
	Response    *siat.ResultadoDocumentoAjuste    `json:"response"`
}

// EmitirDocumentoAjuste emite un documento de ajuste (nota de crédito o débito)
// ante el SIAT.
func (h *SiatHandler) EmitirDocumentoAjuste(w http.ResponseWriter, r *http.Request) {
	if h.siatService == nil {
		http.Error(w, "Servicio SIAT no inicializado", http.StatusServiceUnavailable)
		return
	}

	company, pointOfSale, err := h.loadCompanyAndPointOfSale(r)
	if err != nil {
		http.Error(w, err.Error(), errToStatus(err))
		return
	}

	var body siatDocumentoAjusteRequest
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSONError(w, http.StatusBadRequest, "payload JSON inválido")
			return
		}
	}

	if strings.TrimSpace(body.CufFacturaOriginal) == "" {
		http.Error(w, "cufFacturaOriginal es obligatorio para documentos de ajuste", http.StatusBadRequest)
		return
	}
	if body.TipoNota != 1 && body.TipoNota != 2 {
		http.Error(w, "tipoNota debe ser 1 (nota de crédito) o 2 (nota de débito)", http.StatusBadRequest)
		return
	}

	usuario := "SUPAY"
	if company.UsuarioSiat != "" {
		usuario = company.UsuarioSiat
	}

	req := siat.SolicitudDocumentoAjuste{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         company.CodigoSistema,
		Nit:                   company.Nit,
		Modalidad:             h.modalidad,
		NumeroFactura:         body.NumeroFactura,
		CodigoSucursal:        pointOfSale.CodigoSucursal,
		CodigoPuntoVenta:      pointOfSale.CodigoPuntoVenta,
		Cuis:                  body.Cuis,
		Cufd:                  body.Cufd,
		CodigoControl:         body.CodigoControl,
		FechaEmision:          time.Now().In(siat.LaPaz),
		Usuario:               usuario,
		TipoNota:              siat.TipoNota(body.TipoNota),
		CufFacturaOriginal:    body.CufFacturaOriginal,
		CodigoDocumentoSector: body.CodigoDocumentoSector,
		CodigoTipoFactura:     body.CodigoTipoFactura,
		RazonSocialEmisor:     company.BusinessName,
		Municipio:             company.Municipio,
		Direccion:             company.Direccion,
		Telefono:              &company.Telefono,
		Cliente:               body.Cliente,
		CodigoMetodoPago:      body.CodigoMetodoPago,
		CodigoMoneda:          body.CodigoMoneda,
		TipoCambio:            body.TipoCambio,
		MontoTotal:            body.MontoTotal,
		Leyenda:               body.Leyenda,
		Motivo:                body.Motivo,
		Items:                 body.Items,
	}

	result, err := h.siatService.EmitirDocumentoAjuste(r.Context(), req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(siatDocumentoAjusteResponse{
		Company:     company,
		PointOfSale: pointOfSale,
		Response:    result,
	})
}
