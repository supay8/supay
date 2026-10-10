package app

import (
	"encoding/json"
	"net/http"
	"time"

	deliveryHttp "github.com/brandsrx/supay/internal/delivery/http"
	deliveryModules "github.com/brandsrx/supay/internal/delivery/http/modules"
	"github.com/brandsrx/supay/internal/delivery/http/modules/apikey"
	"github.com/brandsrx/supay/internal/delivery/http/modules/branch"
	"github.com/brandsrx/supay/internal/delivery/http/modules/catalog"
	"github.com/brandsrx/supay/internal/delivery/http/modules/certificate"
	"github.com/brandsrx/supay/internal/delivery/http/modules/company"
	"github.com/brandsrx/supay/internal/delivery/http/modules/customer"
	"github.com/brandsrx/supay/internal/delivery/http/modules/invoice"
	"github.com/brandsrx/supay/internal/delivery/http/modules/pos"
	siatModule "github.com/brandsrx/supay/internal/delivery/http/modules/siat"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/storage"
	"github.com/brandsrx/supay/internal/usecase"
)

type httpApplication struct {
	// HTTP
	modules     []deliveryModules.Module
	router      http.Handler
	server      *http.Server
	authOptions deliveryHttp.AuthOptions
}

func configureRouter(c *App) {
	if c.cfg.RunMode.RunsEmailWorker() {
		c.router = deliveryHttp.NewEmailWorkerRouter(c.emailTaskHandler.SendInvoiceEmail)
		return
	}

	invoiceModule := invoice.NewModule(c.invoiceUsecase, c.invoiceFileService)
	invoiceModule.SetPDFGenerator(c.pdfService)
	c.modules = []deliveryModules.Module{
		company.NewModule(c.companyUsecase),
		apikey.NewModule(c.apiKeyUsecase),
		pos.NewModule(c.pointOfSaleUsecase),
		branch.NewModule(c.branchUsecase),
		customer.NewModule(c.customerUsecase),
		invoiceModule,
		siatModule.NewModule(c.siatUsecase, c.pdfService),
		catalog.NewModule(c.siatUsecase),
		certificate.NewModule(c.certificateUsecase),
	}

	deliveryHttp.SetVerifyAPIKey(postgres.VerifyKey)
	companyCreateHandler := c.companyCreateHandler()
	c.authOptions.SignedDownload = c.signedDownloadHandler()

	c.router = deliveryHttp.NewRouter(c.cfg, c.modules, c.apiKeyRepo, companyCreateHandler, c.authOptions)
}
func configureHTTP(c *App) {
	configureRouter(c)
	writeTimeout := 60 * time.Second
	if c.cfg.RunMode.RunsEmailWorker() {
		writeTimeout = 10 * time.Minute
	}
	c.server = &http.Server{
		Addr:              ":" + c.cfg.Port,
		Handler:           c.router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       120 * time.Second,
	}
}
func (c *App) signedDownloadHandler() http.HandlerFunc {
	if local, ok := c.objectStorage.(*storage.LocalObjectStorage); ok {
		return deliveryHttp.SignedLocalDownload(local)
	}
	return nil
}

func (c *App) companyCreateHandler() http.HandlerFunc {
	uc := c.apiKeyUsecase
	return func(w http.ResponseWriter, r *http.Request) {
		var req usecase.BootstrapCompanyRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			deliveryHttp.RespondValidation(w, "payload JSON inválido")
			return
		}
		resp, err := uc.BootstrapCompany(req)
		if err != nil {
			deliveryHttp.RespondError(w, err)
			return
		}
		deliveryHttp.WriteJSON(w, http.StatusCreated, resp)
	}
}
