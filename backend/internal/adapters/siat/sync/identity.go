package sync

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	goSiat "github.com/ron86i/go-siat/v2"
)

func parseNit(nit string) int64 { n, _ := strconv.ParseInt(strings.TrimSpace(nit), 10, 64); return n }
func withDynamicConfig(ctx context.Context, base goSiat.Config, ambiente int, sistema, nit string) context.Context {
	return WithDynamicConfig(ctx, base, ambiente, sistema, nit)
}
func applyIdentityValues(cfg goSiat.Config, ambiente *int, sistema, nit *string) error {
	return ApplyIdentityValues(cfg, ambiente, sistema, nit)
}

// withDynamicConfig sobreescribe la identidad del contribuyente (NIT, sistema,
// ambiente) por empresa en el contexto de la petición, sin tocar la config global.
func WithDynamicConfig(ctx context.Context, base goSiat.Config, ambiente int, sistema, nit string) context.Context {
	// The SDK identity is configured once in goSiat.Config. Keep the arguments
	// for source compatibility with older callers, but never replace the global
	// identity per request.
	return goSiat.WithDynamicConfig(ctx, base)
}

func ApplyIdentityValues(cfg goSiat.Config, ambiente *int, sistema *string, nit *string) error {
	if *ambiente == 0 {
		*ambiente = cfg.CodigoAmbiente
	} else if *ambiente != cfg.CodigoAmbiente {
		return fmt.Errorf("siat identidad: codigoAmbiente de la solicitud (%d) no coincide con Config (%d)", *ambiente, cfg.CodigoAmbiente)
	}
	if strings.TrimSpace(*sistema) == "" {
		*sistema = cfg.CodigoSistema
	} else if strings.TrimSpace(*sistema) != strings.TrimSpace(cfg.CodigoSistema) {
		return fmt.Errorf("siat identidad: codigoSistema de la solicitud no coincide con Config")
	}
	if strings.TrimSpace(*nit) == "" {
		*nit = strconv.FormatInt(cfg.Nit, 10)
	} else if parsed := parseNit(*nit); parsed != cfg.Nit {
		return fmt.Errorf("siat identidad: nit de la solicitud no coincide con Config")
	}
	return nil
}
