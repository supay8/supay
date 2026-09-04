package siat

import (
	"context"
	"fmt"

	"github.com/ron86i/go-siat/v2/pkg/models"
)

// Este archivo centraliza el enrutamiento SOAP del SDK: cada operación de
// facturación se ejecuta en la fachada que atiende al documento-sector según el
// catálogo normativo (CompraVenta para 1/35/41, Telecomunicaciones para 22/49,
// ServicioBasico para 13/40, EntidadFinanciera para 15, BoletoAereo para 30;
// el resto por modalidad en Electronica/Computarizada). Enviar un sector por la
// fachada equivocada produce el rechazo 932 del SIAT.
//
// Los documentos de ajuste (24, 29, 47, 48) no pasan por aquí: usan las
// operaciones propias del servicio DocumentoAjuste (ver documento_ajuste.go).

func envioPorModalidad(s *Service, modalidad int) (electronico, computarizado bool) {
	return modalidad != ModalidadComputarizada, modalidad == ModalidadComputarizada
}

// recepcionFacturaParaPerfil envía recepcionFactura por la fachada del perfil.
func (s *Service) recepcionFacturaParaPerfil(ctx context.Context, p *SectorProfile, modalidad int, req models.RecepcionFactura) (any, error) {
	switch p.Facade.Fixed() {
	case FachadaCompraVenta:
		return s.sdk.CompraVenta().RecepcionFactura(ctx, req)
	case FachadaTelecomunicaciones:
		return s.sdk.Telecomunicaciones().RecepcionFactura(ctx, req)
	case FachadaServicioBasico:
		return s.sdk.ServicioBasico().RecepcionFactura(ctx, req)
	case FachadaEntidadFinanciera:
		return s.sdk.EntidadFinanciera().RecepcionFactura(ctx, req)
	case FachadaBoletoAereo:
		return nil, fmt.Errorf("siat sectores %d (%s): el SIAT no recibe boletos aéreos individuales; use la emisión masiva", p.Codigo, p.Nombre)
	default:
		envio := s.sdk.Electronica()
		if modalidad == ModalidadComputarizada {
			envio = s.sdk.Computarizada()
		}
		return envio.RecepcionFactura(ctx, req)
	}
}

func (s *Service) verificacionEstadoParaPerfil(ctx context.Context, p *SectorProfile, modalidad int, req models.VerificacionEstadoFactura) (any, error) {
	switch p.Facade.Fixed() {
	case FachadaCompraVenta:
		return s.sdk.CompraVenta().VerificacionEstadoFactura(ctx, req)
	case FachadaTelecomunicaciones:
		return s.sdk.Telecomunicaciones().VerificacionEstadoFactura(ctx, req)
	case FachadaServicioBasico:
		return s.sdk.ServicioBasico().VerificacionEstadoFactura(ctx, req)
	case FachadaEntidadFinanciera:
		return s.sdk.EntidadFinanciera().VerificacionEstadoFactura(ctx, req)
	case FachadaBoletoAereo:
		return s.sdk.BoletoAereo().VerificacionEstadoFactura(ctx, req)
	default:
		envio := s.sdk.Electronica()
		if modalidad == ModalidadComputarizada {
			envio = s.sdk.Computarizada()
		}
		return envio.VerificacionEstadoFactura(ctx, req)
	}
}

func (s *Service) anulacionParaPerfil(ctx context.Context, p *SectorProfile, modalidad int, req models.AnulacionFactura) (any, error) {
	switch p.Facade.Fixed() {
	case FachadaCompraVenta:
		return s.sdk.CompraVenta().AnulacionFactura(ctx, req)
	case FachadaTelecomunicaciones:
		return s.sdk.Telecomunicaciones().AnulacionFactura(ctx, req)
	case FachadaServicioBasico:
		return s.sdk.ServicioBasico().AnulacionFactura(ctx, req)
	case FachadaEntidadFinanciera:
		return s.sdk.EntidadFinanciera().AnulacionFactura(ctx, req)
	case FachadaBoletoAereo:
		return s.sdk.BoletoAereo().AnulacionFactura(ctx, req)
	default:
		envio := s.sdk.Electronica()
		if modalidad == ModalidadComputarizada {
			envio = s.sdk.Computarizada()
		}
		return envio.AnulacionFactura(ctx, req)
	}
}

func (s *Service) reversionParaPerfil(ctx context.Context, p *SectorProfile, modalidad int, req models.ReversionAnulacionFactura) (any, error) {
	switch p.Facade.Fixed() {
	case FachadaCompraVenta:
		return s.sdk.CompraVenta().ReversionAnulacionFactura(ctx, req)
	case FachadaTelecomunicaciones:
		return s.sdk.Telecomunicaciones().ReversionAnulacionFactura(ctx, req)
	case FachadaServicioBasico:
		return s.sdk.ServicioBasico().ReversionAnulacionFactura(ctx, req)
	case FachadaEntidadFinanciera:
		return s.sdk.EntidadFinanciera().ReversionAnulacionFactura(ctx, req)
	case FachadaBoletoAereo:
		return s.sdk.BoletoAereo().ReversionAnulacionFactura(ctx, req)
	default:
		envio := s.sdk.Electronica()
		if modalidad == ModalidadComputarizada {
			envio = s.sdk.Computarizada()
		}
		return envio.ReversionAnulacionFactura(ctx, req)
	}
}

func (s *Service) paqueteParaPerfil(ctx context.Context, p *SectorProfile, modalidad int, req models.RecepcionPaqueteFactura) (any, error) {
	if p.Facade.Fixed() == FachadaBoletoAereo {
		return nil, fmt.Errorf("siat sectores %d (%s): el boleto aéreo no admite paquete de facturas; use la emisión masiva", p.Codigo, p.Nombre)
	}
	switch p.Facade.Fixed() {
	case FachadaCompraVenta:
		return s.sdk.CompraVenta().RecepcionPaqueteFactura(ctx, req)
	case FachadaTelecomunicaciones:
		return s.sdk.Telecomunicaciones().RecepcionPaqueteFactura(ctx, req)
	case FachadaServicioBasico:
		return s.sdk.ServicioBasico().RecepcionPaqueteFactura(ctx, req)
	case FachadaEntidadFinanciera:
		return s.sdk.EntidadFinanciera().RecepcionPaqueteFactura(ctx, req)
	default:
		envio := s.sdk.Electronica()
		if modalidad == ModalidadComputarizada {
			envio = s.sdk.Computarizada()
		}
		return envio.RecepcionPaqueteFactura(ctx, req)
	}
}

func (s *Service) validacionPaqueteParaPerfil(ctx context.Context, p *SectorProfile, modalidad int, req models.ValidacionRecepcionPaqueteFactura) (any, error) {
	if p.Facade.Fixed() == FachadaBoletoAereo {
		return nil, fmt.Errorf("siat sectores %d (%s): el boleto aéreo no admite paquete de facturas", p.Codigo, p.Nombre)
	}
	switch p.Facade.Fixed() {
	case FachadaCompraVenta:
		return s.sdk.CompraVenta().ValidacionRecepcionPaqueteFactura(ctx, req)
	case FachadaTelecomunicaciones:
		return s.sdk.Telecomunicaciones().ValidacionRecepcionPaqueteFactura(ctx, req)
	case FachadaServicioBasico:
		return s.sdk.ServicioBasico().ValidacionRecepcionPaqueteFactura(ctx, req)
	case FachadaEntidadFinanciera:
		return s.sdk.EntidadFinanciera().ValidacionRecepcionPaqueteFactura(ctx, req)
	default:
		envio := s.sdk.Electronica()
		if modalidad == ModalidadComputarizada {
			envio = s.sdk.Computarizada()
		}
		return envio.ValidacionRecepcionPaqueteFactura(ctx, req)
	}
}

func (s *Service) masivaParaPerfil(ctx context.Context, p *SectorProfile, modalidad int, req models.RecepcionMasivaFactura) (any, error) {
	switch p.Facade.Fixed() {
	case FachadaCompraVenta:
		return s.sdk.CompraVenta().RecepcionMasivaFactura(ctx, req)
	case FachadaTelecomunicaciones:
		return s.sdk.Telecomunicaciones().RecepcionMasivaFactura(ctx, req)
	case FachadaServicioBasico:
		return s.sdk.ServicioBasico().RecepcionMasivaFactura(ctx, req)
	case FachadaEntidadFinanciera:
		return s.sdk.EntidadFinanciera().RecepcionMasivaFactura(ctx, req)
	case FachadaBoletoAereo:
		return s.sdk.BoletoAereo().RecepcionMasivaFactura(ctx, req)
	default:
		envio := s.sdk.Electronica()
		if modalidad == ModalidadComputarizada {
			envio = s.sdk.Computarizada()
		}
		return envio.RecepcionMasivaFactura(ctx, req)
	}
}

func (s *Service) validacionMasivaParaPerfil(ctx context.Context, p *SectorProfile, modalidad int, req models.ValidacionRecepcionMasivaFactura) (any, error) {
	switch p.Facade.Fixed() {
	case FachadaCompraVenta:
		return s.sdk.CompraVenta().ValidacionRecepcionMasivaFactura(ctx, req)
	case FachadaTelecomunicaciones:
		return s.sdk.Telecomunicaciones().ValidacionRecepcionMasivaFactura(ctx, req)
	case FachadaServicioBasico:
		return s.sdk.ServicioBasico().ValidacionRecepcionMasivaFactura(ctx, req)
	case FachadaEntidadFinanciera:
		return s.sdk.EntidadFinanciera().ValidacionRecepcionMasivaFactura(ctx, req)
	case FachadaBoletoAereo:
		return s.sdk.BoletoAereo().ValidacionRecepcionMasivaFactura(ctx, req)
	default:
		envio := s.sdk.Electronica()
		if modalidad == ModalidadComputarizada {
			envio = s.sdk.Computarizada()
		}
		return envio.ValidacionRecepcionMasivaFactura(ctx, req)
	}
}

// recepcionDocumentoAjusteEnvio envía recepcionDocumentoAjuste y las operaciones
// de ciclo de vida de notas por el servicio DocumentoAjuste (único endpoint que
// las recibe, sin importar la modalidad).
func (s *Service) recepcionDocumentoAjusteEnvio(ctx context.Context, req models.RecepcionDocumentoAjuste) (any, error) {
	return s.sdk.DocumentoAjuste().RecepcionDocumentoAjuste(ctx, req)
}
