package app

import (
	"errors"
	"net/http"

	appconfig "github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/emissionqueue"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/usecase"
	"gorm.io/gorm"
)

type App struct {
	cfg appconfig.Config
	db  *gorm.DB
	repositories
	infrastructure
	services
	httpApplication
	closers []func() error
}

// Close releases only resources owned by this application.
func (a *App) Close() error {
	var result error
	for i := len(a.closers) - 1; i >= 0; i-- {
		result = errors.Join(result, a.closers[i]())
	}
	a.closers = nil
	return result
}
func (a *App) Server() *http.Server                            { return a.server }
func (a *App) InvoiceRepo() ports.InvoiceRepository            { return a.invoiceRepo }
func (a *App) MaintenanceService() *usecase.MaintenanceService { return a.maintenanceService }
func (a *App) EmissionQueue() *emissionqueue.Service           { return a.emissionQueue }
