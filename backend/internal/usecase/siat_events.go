package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/domain/fiscal"
	"github.com/brandsrx/supay/internal/ports"
)

type EventoSignificativoInput struct {
	CodigoMotivoEvento    int    `json:"codigo_motivo_evento"`
	Descripcion           string `json:"descripcion"`
	CufdEvento            string `json:"cufd_evento"`
	FechaHoraInicioEvento string `json:"fecha_hora_inicio_evento"`
	FechaHoraFinEvento    string `json:"fecha_hora_fin_evento"`
}

type EventoSignificativoResultado struct {
	Company     *domain.Company
	PointOfSale *domain.PointOfSale
	Response    *ports.FiscalEventResult
}

// RegistrarEventoSignificativo registra una contingencia ante el SIAT
// (registroEventoSignificativo). El CUFD vigente se usa como cufdEvento salvo
// que el input lo sobrescriba (p.ej. el CUFD vencido durante la contingencia).
func (uc *SiatUsecase) RegistrarEventoSignificativo(ctx context.Context, companyID, posID string, body EventoSignificativoInput) (*EventoSignificativoResultado, error) {
	if err := uc.requireService(); err != nil {
		return nil, err
	}
	company, pointOfSale, err := uc.LoadCompanyAndPointOfSale(companyID, posID)
	if err != nil {
		return nil, err
	}
	if pointOfSale.Cuis == nil || *pointOfSale.Cuis == "" {
		return nil, domain.NewConflictError("El punto de venta no tiene CUIS activo")
	}
	cufd, err := uc.cufdRepo.GetActiveByPos(pointOfSale.ID)
	if err != nil {
		return nil, domain.NewConflictError("El punto de venta no tiene CUFD vigente")
	}

	codigoMotivo := body.CodigoMotivoEvento
	if codigoMotivo <= 0 {
		codigoMotivo = fiscal.MotivoCorteInternet
	}
	descripcion := strings.TrimSpace(body.Descripcion)
	if descripcion == "" {
		descripcion = "Corte del servicio de internet"
	}
	cufdEvento := strings.TrimSpace(body.CufdEvento)
	if cufdEvento == "" {
		cufdEvento = cufd.Cufd
	}
	var pendingLocalEvent *domain.ContingencyEvent
	if uc.contingencyRepo != nil {
		if latest, latestErr := uc.contingencyRepo.GetLatestByPointOfSale(pointOfSale.ID); latestErr == nil && latest != nil && !latest.IsSynced && latest.EndDate == nil {
			pendingLocalEvent = latest
		}
	}

	var inicio, fin time.Time
	var errInicio, errFin error
	inicioStr := strings.TrimSpace(body.FechaHoraInicioEvento)
	finStr := strings.TrimSpace(body.FechaHoraFinEvento)
	ambasVacias := inicioStr == "" && finStr == ""
	algunaVacia := inicioStr == "" || finStr == ""
	if ambasVacias && pendingLocalEvent != nil {
		// Completa el evento abierto automáticamente cuando falló POST /emit.
		// El inicio debe abarcar las facturas offline ya vinculadas al evento.
		inicio = pendingLocalEvent.StartDate
		fin = time.Now().In(fiscal.LaPaz)
		if !fin.After(inicio) {
			fin = inicio.Add(time.Second)
		}
	} else if ambasVacias {
		// Caso holgada intencional: cliente no envió fechas -> generar now-10m → now+1h50m
		errInicio = fmt.Errorf("vacía")
		errFin = fmt.Errorf("vacía")
	} else if algunaVacia {
		// Si solo una viene, es error del cliente, no generar holgada silenciosa
		if inicioStr == "" {
			return nil, domain.NewBadRequestError("fechaHoraInicioEvento es obligatoria si se envía fechaHoraFinEvento")
		}
		return nil, domain.NewBadRequestError("fechaHoraFinEvento es obligatoria si se envía fechaHoraInicioEvento")
	} else {
		inicio, errInicio = ParseFechaSiat(inicioStr)
		if errInicio != nil {
			return nil, domain.NewBadRequestError("fechaHoraInicioEvento inválida (use YYYY-MM-DDTHH:mm:ss.SSS)")
		}
		fin, errFin = ParseFechaSiat(finStr)
		if errFin != nil {
			return nil, domain.NewBadRequestError("fechaHoraFinEvento inválida (use YYYY-MM-DDTHH:mm:ss.SSS)")
		}
		if !fin.After(inicio) {
			return nil, domain.NewBadRequestError("fechaHoraFinEvento debe ser posterior a fechaHoraInicioEvento")
		}
	}

	// Solo aplicar ventana holgada si AMBAS estaban vacías (caso anterior)
	if fiscal.DebeUsarVentanaHolgada(inicio, fin, errInicio, errFin) {
		prevInicio, prevFin := inicio, fin
		inicio, fin = fiscal.VentanaContingenciaHolgada(time.Now())
		// Clamp a vigencia CUFD para no exceder límite SIAT
		if !cufd.ValidTo.IsZero() && fin.After(cufd.ValidTo) {
			fin = cufd.ValidTo
			// Mantener duración 2h si es posible recortando inicio
			candidateInicio := fin.Add(-2 * time.Hour)
			if !cufd.ValidFrom.IsZero() && candidateInicio.Before(cufd.ValidFrom) {
				candidateInicio = cufd.ValidFrom
			}
			inicio = candidateInicio
		}
		if !cufd.ValidFrom.IsZero() && inicio.Before(cufd.ValidFrom) {
			inicio = cufd.ValidFrom
		}
		slog.Warn("contingencia: ventana holgada aplicada para evitar error 1040",
			"prev_inicio", prevInicio, "prev_fin", prevFin,
			"nuevo_inicio", inicio.In(fiscal.LaPaz).Format(time.RFC3339),
			"nuevo_fin", fin.In(fiscal.LaPaz).Format(time.RFC3339),
			"duracion", fin.Sub(inicio).String(),
		)
	}

	req := ports.FiscalEvent{
		CodigoAmbiente:        company.Ambiente.CodigoAmbiente(),
		CodigoSistema:         "",
		Nit:                   company.Nit,
		CodigoSucursal:        pointOfSale.CodigoSucursal,
		CodigoPuntoVenta:      resolveCodigoPuntoVenta(pointOfSale),
		Cuis:                  *pointOfSale.Cuis,
		Cufd:                  cufd.Cufd,
		CufdEvento:            cufdEvento,
		CodigoMotivoEvento:    codigoMotivo,
		Descripcion:           descripcion,
		FechaHoraInicioEvento: inicio,
		FechaHoraFinEvento:    fin,
	}
	svc, err := uc.resolveService(ctx, company.ID)
	if err != nil {
		return nil, err
	}
	result, err := svc.RegisterSignificantEvent(ctx, req)
	if err != nil {
		return nil, err
	}

	// Se persiste el codigoRecepcion en el mismo evento local que agrupa las
	// facturas offline. Si no habia uno abierto, se crea el evento normalmente.
	if uc.contingencyRepo != nil && result.Transaccion && result.CodigoRecepcion != "" {
		ev := pendingLocalEvent
		if ev == nil {
			ev = &domain.ContingencyEvent{PointOfSaleID: pointOfSale.ID}
		}
		ev.Reason = motivoEventoAReason(codigoMotivo)
		ev.Description = &descripcion
		ev.StartDate = inicio
		ev.EndDate = &fin
		ev.SiatEventCode = &result.CodigoRecepcion
		ev.IsSynced = true
		persist := uc.contingencyRepo.Create
		if pendingLocalEvent != nil {
			persist = uc.contingencyRepo.Update
		}
		if err := persist(ev); err != nil {
			// El evento SIAT ya se registró exitosamente; la persistencia local es
			// complementaria. Un error aquí implica que la resolución automática de
			// codigoEvento no funcionará para envíos posteriores de paquetes.
			slog.Error("no se pudo persistir evento de contingencia",
				"siat_code", result.CodigoRecepcion, "pos_id", pointOfSale.ID, "error", err)
		}
	}

	return &EventoSignificativoResultado{Company: company, PointOfSale: pointOfSale, Response: &result}, nil
}

// motivoEventoAReason traduce el codigoMotivoEvento del catálogo del SIAT a la
// razón de contingencia local (razón definida por el dominio).
func motivoEventoAReason(motivo int) string {
	if motivo == fiscal.MotivoCorteInternet {
		return "FALLA_CONEXION_INTERNET"
	}
	return "OTRO"
}
