package app

import (
	"testing"

	"github.com/brandsrx/supay/internal/adapters/siat/sandbox"
	"github.com/brandsrx/supay/internal/config"
)

func TestFiscalServiceDoesNotFallbackToSandbox(t *testing.T) {
	c := NewContainer(config.Config{}, nil)

	if got := c.FiscalService(); got != nil {
		t.Fatalf("FiscalService() = %T; want nil without real credentials", got)
	}
}

func TestFiscalServiceUsesSandboxOnlyWhenExplicitlyEnabled(t *testing.T) {
	c := NewContainer(config.Config{SiatSandbox: true}, nil)

	if _, ok := c.FiscalService().(*sandbox.FiscalService); !ok {
		t.Fatalf("FiscalService() = %T; want *sandbox.FiscalService", c.FiscalService())
	}
}
