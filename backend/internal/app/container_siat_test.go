package app

import (
	"testing"

	"github.com/brandsrx/supay/internal/adapters/siat/sandbox"
	"github.com/brandsrx/supay/internal/config"
)

func TestFiscalServiceDoesNotFallbackToSandbox(t *testing.T) {
	c := &App{cfg: config.Config{StorageDriver: "none"}}
	if err := configureSIAT(c); err != nil {
		t.Fatal(err)
	}

	if got := c.fiscalService; got != nil {
		t.Fatalf("FiscalService() = %T; want nil without real credentials", got)
	}
}

func TestFiscalServiceUsesSandboxOnlyWhenExplicitlyEnabled(t *testing.T) {
	c := &App{cfg: config.Config{SiatSandbox: true}}
	if err := configureSIAT(c); err != nil {
		t.Fatal(err)
	}

	if _, ok := c.fiscalService.(*sandbox.FiscalService); !ok {
		t.Fatalf("FiscalService() = %T; want *sandbox.FiscalService", c.fiscalService)
	}
}
