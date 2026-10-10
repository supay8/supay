package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/brandsrx/supay/internal/domain/fiscal"
	goSiat "github.com/ron86i/go-siat/v2"
	"github.com/ron86i/go-siat/v2/pkg/models"
)

// cuisYaVigente detecta el mensaje 980 del SIAT ("EXISTE UN CUIS VIGENTE PARA
// LA SUCURSAL O PUNTO DE VENTA"). En ese caso el SIAT responde
// transaccion=false pero incluye el CUIS vigente en <codigo>: no es un fallo,
// es la reemisión del código existente.
func cuisYaVigente(err error) bool {
	var siatErr *goSiat.SiatError
	return errors.As(err, &siatErr) && siatErr.SiatCode == goSiat.CodeExisteCuisVigente
}

// SolicitarCUIS solicita un CUIS al SIAT usando el SDK go-siat.
func (s *Service) SolicitarCUIS(ctx context.Context, req SolicitudCuis) (*RespuestaCuis, error) {
	if err := applyIdentityValues(s.sdk.Config(), &req.CodigoAmbiente, &req.CodigoSistema, &req.Nit); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	request := models.NewCuisBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoModalidad(req.CodigoModalidad).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.Codigos().SolicitudCuis(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat cuis: %w", err)
	}

	result := resp.Body.Content.RespuestaCuis

	if err := goSiat.Verify(result); err != nil {
		// 980 con código presente: éxito — el propio SIAT devuelve el CUIS
		// vigente en la misma respuesta; se normaliza como transacción OK y
		// se conservan los mensajes para trazabilidad.
		if !cuisYaVigente(err) || result.Codigo == "" {
			return nil, fmt.Errorf("siat cuis: %w", err)
		}
		result.Transaccion = true
	}

	return &RespuestaCuis{
		Codigo:        result.Codigo,
		FechaVigencia: XMLDateTime{Time: fiscal.SIATWallClockToInstant(result.FechaVigencia)},
		Transaccion:   result.Transaccion,
		Mensajes:      toMensajes(result.MensajesList),
	}, nil
}

// SolicitarCUFD solicita un CUFD al SIAT usando el SDK go-siat.
func (s *Service) SolicitarCUFD(ctx context.Context, req SolicitudCufd) (*RespuestaCufd, error) {
	if err := applyIdentityValues(s.sdk.Config(), &req.CodigoAmbiente, &req.CodigoSistema, &req.Nit); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	request := models.NewCufdBuilder().
		WithCodigoSucursal(req.CodigoSucursal).
		WithCodigoPuntoVenta(req.CodigoPuntoVenta).
		WithCodigoModalidad(req.CodigoModalidad).
		WithCuis(req.Cuis).
		Build()

	ctx = withDynamicConfig(ctx, s.sdk.Config(), req.CodigoAmbiente, req.CodigoSistema, req.Nit)

	resp, err := s.sdk.Codigos().SolicitudCufd(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("siat cufd: %w", err)
	}
	if err := goSiat.Verify(resp.Body.Content.RespuestaCufd); err != nil {
		return nil, fmt.Errorf("siat cufd: %w", err)
	}

	result := resp.Body.Content.RespuestaCufd
	return &RespuestaCufd{
		Codigo:        result.Codigo,
		CodigoControl: result.CodigoControl,
		Direccion:     result.Direccion,
		FechaVigencia: XMLDateTime{Time: fiscal.SIATWallClockToInstant(result.FechaVigencia)},
		Transaccion:   result.Transaccion,
		Mensajes:      toMensajes(result.MensajesList),
	}, nil
}
