package usecase

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/siat"
)

// In-memory fake repos
type memBranchRepo struct{ m map[string]*domain.Branch }

func (r *memBranchRepo) Create(b *domain.Branch) error { b.ID = "branch-1"; r.m[b.ID] = b; return nil }
func (r *memBranchRepo) GetByID(id string) (*domain.Branch, error) {
	if v, ok := r.m[id]; ok {
		return v, nil
	}
	return nil, ErrNotFound
}
func (r *memBranchRepo) List(companyID string) ([]*domain.Branch, error) { return nil, nil }
func (r *memBranchRepo) Update(b *domain.Branch) error                   { r.m[b.ID] = b; return nil }
func (r *memBranchRepo) Delete(id string) error                          { delete(r.m, id); return nil }
func (r *memBranchRepo) GetByCompanyAndSucursal(companyID string, codigoSucursal int) (*domain.Branch, error) {
	for _, b := range r.m {
		if b.CompanyID == companyID && b.CodigoSucursal == codigoSucursal {
			return b, nil
		}
	}
	return nil, ErrNotFound
}

type memCompanyRepo struct{ m map[string]*domain.Company }

func (r *memCompanyRepo) Create(c *domain.Company) error {
	c.ID = "company-1"
	r.m[c.ID] = c
	return nil
}
func (r *memCompanyRepo) GetByNit(nit string) (*domain.Company, error) {
	for _, v := range r.m {
		if v.Nit == nit {
			return v, nil
		}
	}
	return nil, ErrNotFound
}
func (r *memCompanyRepo) GetByID(id string) (*domain.Company, error) {
	if v, ok := r.m[id]; ok {
		return v, nil
	}
	return nil, ErrNotFound
}
func (r *memCompanyRepo) Update(c *domain.Company) error { r.m[c.ID] = c; return nil }
func (r *memCompanyRepo) Delete(id string) error         { delete(r.m, id); return nil }

type memPosRepo struct {
	m map[string]*domain.PointOfSale
}

func (r *memPosRepo) Create(p *domain.PointOfSale) error { p.ID = "pos-1"; r.m[p.ID] = p; return nil }
func (r *memPosRepo) GetByID(id string) (*domain.PointOfSale, error) {
	if v, ok := r.m[id]; ok {
		return v, nil
	}
	return nil, ErrNotFound
}
func (r *memPosRepo) List(companyID string) ([]*domain.PointOfSale, error) { return nil, nil }
func (r *memPosRepo) ListByBranch(branchID string) ([]*domain.PointOfSale, error) {
	out := []*domain.PointOfSale{}
	for _, p := range r.m {
		if p.BranchId != nil && *p.BranchId == branchID {
			out = append(out, p)
		}
	}
	return out, nil
}
func (r *memPosRepo) Update(p *domain.PointOfSale) error { r.m[p.ID] = p; return nil }
func (r *memPosRepo) Delete(id string) error             { delete(r.m, id); return nil }

type memCuisRepo struct{ m map[string]*domain.Cuis }

func (r *memCuisRepo) Create(c *domain.Cuis) error {
	c.ID = "cuis-1"
	r.m[c.PointOfSaleID] = c
	return nil
}
func (r *memCuisRepo) GetActiveByPos(pointOfSaleID string) (*domain.Cuis, error) {
	if v, ok := r.m[pointOfSaleID]; ok {
		return v, nil
	}
	return nil, ErrNotFound
}
func (r *memCuisRepo) DeactivateExpired() error { return nil }

type memCufdRepo struct{ m map[string]*domain.Cufd }

func (r *memCufdRepo) Create(c *domain.Cufd) error {
	c.ID = "cufd-1"
	r.m[c.PointOfSaleID] = c
	return nil
}
func (r *memCufdRepo) GetActiveByPos(pointOfSaleID string) (*domain.Cufd, error) {
	if v, ok := r.m[pointOfSaleID]; ok {
		return v, nil
	}
	return nil, ErrNotFound
}
func (r *memCufdRepo) DeactivateExpired() error { return nil }

type memTipoPVRepo struct {
	m map[string][]*domain.TipoPuntoVenta
}

func (r *memTipoPVRepo) Replace(companyID string, tipos []domain.TipoPuntoVenta, syncedAt time.Time) error {
	list := make([]*domain.TipoPuntoVenta, 0, len(tipos))
	for _, t := range tipos {
		tc := t
		list = append(list, &tc)
	}
	r.m[companyID] = list
	return nil
}
func (r *memTipoPVRepo) List(companyID string) ([]*domain.TipoPuntoVenta, error) {
	if v, ok := r.m[companyID]; ok {
		return v, nil
	}
	return nil, nil
}
func (r *memTipoPVRepo) FindByClasificador(companyID string, codigoClasificador int) (*domain.TipoPuntoVenta, error) {
	list, ok := r.m[companyID]
	if !ok {
		return nil, ErrNotFound
	}
	for _, t := range list {
		if t.CodigoClasificador == codigoClasificador {
			return t, nil
		}
	}
	return nil, ErrNotFound
}

// minimal sentinel
var ErrNotFound = &domainErr{"not found"}

type domainErr struct{ s string }

func (e *domainErr) Error() string { return e.s }

func mockSiatServer(t *testing.T, registerOK bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		b := string(raw)
		w.Header().Set("Content-Type", "text/xml; charset=utf-8")
		if strings.Contains(b, "sincronizarParametricaTipoPuntoVenta") {
			resp := `<?xml version="1.0" encoding="UTF-8"?>
                <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
                  <soapenv:Body>
                    <sincronizarParametricaTipoPuntoVentaResponse>
                      <RespuestaListaParametricas>
                        <transaccion>true</transaccion>
                        <listaCodigos>
                          <codigoClasificador>1</codigoClasificador>
                          <descripcion>Punto de Venta Fisico</descripcion>
                        </listaCodigos>
                        <listaCodigos>
                          <codigoClasificador>2</codigoClasificador>
                          <descripcion>Punto de Venta Movil</descripcion>
                        </listaCodigos>
                      </RespuestaListaParametricas>
                    </sincronizarParametricaTipoPuntoVentaResponse>
                  </soapenv:Body>
                </soapenv:Envelope>`
			_, _ = w.Write([]byte(resp))
			return
		}
		if strings.Contains(b, "registroPuntoVenta") {
			var resp string
			if registerOK {
				resp = `<?xml version="1.0" encoding="UTF-8"?>
                <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
                  <soapenv:Body>
                    <registroPuntoVentaResponse>
                      <RespuestaRegistroPuntoVenta>
                        <codigoPuntoVenta>123</codigoPuntoVenta>
                        <transaccion>true</transaccion>
                      </RespuestaRegistroPuntoVenta>
                    </registroPuntoVentaResponse>
                  </soapenv:Body>
                </soapenv:Envelope>`
			} else {
				resp = `<?xml version="1.0" encoding="UTF-8"?>
                <soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/">
                  <soapenv:Body>
                    <registroPuntoVentaResponse>
                      <RespuestaRegistroPuntoVenta>
                        <codigoPuntoVenta>0</codigoPuntoVenta>
                        <transaccion>false</transaccion>
                        <mensajesList>
                          <codigo>ERROR-1</codigo>
                          <descripcion>Punto de venta rechazado por el SIAT</descripcion>
                        </mensajesList>
                      </RespuestaRegistroPuntoVenta>
                    </registroPuntoVentaResponse>
                  </soapenv:Body>
                </soapenv:Envelope>`
			}
			_, _ = w.Write([]byte(resp))
			return
		}
		if strings.Contains(b, "SolicitudCuis") {
			resp := `<?xml version="1.0" encoding="UTF-8"?>
                <Envelope>
                  <Body>
                    <cuisResponse>
                      <RespuestaCuis>
                        <codigo>CUIS-ABC-123</codigo>
                        <fechaVigencia>2026-12-31T23:59:59</fechaVigencia>
                      </RespuestaCuis>
                    </cuisResponse>
                  </Body>
                </Envelope>`
			_, _ = w.Write([]byte(resp))
			return
		}
		if strings.Contains(b, "SolicitudCufd") {
			resp := `<?xml version="1.0" encoding="UTF-8"?>
                <Envelope>
                  <Body>
                    <cufdResponse>
                      <RespuestaCufd>
                        <codigo>CUFD-XYZ-789</codigo>
                        <codigoControl>CTRL-001</codigoControl>
                        <fechaVigencia>2026-12-31T23:59:59</fechaVigencia>
                      </RespuestaCufd>
                    </cufdResponse>
                  </Body>
                </Envelope>`
			_, _ = w.Write([]byte(resp))
			return
		}
		// default
		w.WriteHeader(500)
		_, _ = w.Write([]byte("unknown request"))
	}))
}

func newProvisionService(srvURL string, tipoPVRepo domain.TipoPuntoVentaRepository) (*PointOfSaleProvisionService, *memBranchRepo, *memCompanyRepo, *memPosRepo, *memCuisRepo, *memCufdRepo) {
	cfg := siat.Config{EndpointURL: srvURL, WSDLURL: srvURL + "?wsdl", Timeout: 5 * time.Second, Headers: map[string]string{"apikey": "TokenApi FAKE"}}
	client, err := siat.NewClient(cfg)
	if err != nil {
		panic(err)
	}

	cuisSvc := siat.NewCuisService(client)
	cufdSvc := siat.NewCufdService(client)

	branchRepo := &memBranchRepo{m: map[string]*domain.Branch{}}
	companyRepo := &memCompanyRepo{m: map[string]*domain.Company{}}
	posRepo := &memPosRepo{m: map[string]*domain.PointOfSale{}}
	cuisRepo := &memCuisRepo{m: map[string]*domain.Cuis{}}
	cufdRepo := &memCufdRepo{m: map[string]*domain.Cufd{}}

	svc := NewPointOfSaleProvisionService(branchRepo, posRepo, companyRepo, client, cuisSvc, cufdSvc, cuisRepo, cufdRepo, tipoPVRepo, 1)

	comp := &domain.Company{ID: "company-1", Nit: "9971522011", CodigoSistema: "CODE-TEST", Ambiente: domain.EnvironmentPiloto}
	_ = companyRepo.Create(comp)
	br := &domain.Branch{ID: "branch-1", CompanyID: comp.ID, CodigoSucursal: 1, Name: "B1", Active: true}
	_ = branchRepo.Create(br)

	return svc, branchRepo, companyRepo, posRepo, cuisRepo, cufdRepo
}

func TestProvision_WithMockedSiatServer(t *testing.T) {
	srv := mockSiatServer(t, true)
	defer srv.Close()

	tipoPVRepo := &memTipoPVRepo{m: map[string][]*domain.TipoPuntoVenta{}}
	svc, _, _, posRepo, cuisRepo, cufdRepo := newProvisionService(srv.URL, tipoPVRepo)

	input := CreatePOSInput{Description: "desc", Name: "POS-1", CodigoTipoPuntoVenta: 2}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pos, err := svc.CreateOperationalPOS(ctx, "branch-1", input)
	if err != nil {
		t.Fatalf("provision error: %v", err)
	}

	if pos.Status != "OPERATIVE" {
		t.Fatalf("expected OPERATIVE, got %s", pos.Status)
	}
	if pos.SiatCode == nil || *pos.SiatCode != 123 {
		t.Fatalf("expected official SIAT code 123, got %v", pos.SiatCode)
	}
	if pos.TipoPuntoVenta == nil || *pos.TipoPuntoVenta != 2 {
		t.Fatalf("expected tipo 2, got %v", pos.TipoPuntoVenta)
	}
	if !pos.SiatTransaccion {
		t.Fatalf("expected SiatTransaccion=true")
	}
	if pos.SiatRegisteredAt == nil {
		t.Fatalf("expected SiatRegisteredAt set")
	}
	if pos.SiatResponse == nil {
		t.Fatalf("expected SiatResponse persisted")
	}

	// verify catalog was synced and persisted
	tipos, _ := tipoPVRepo.List("company-1")
	if len(tipos) != 2 {
		t.Fatalf("expected 2 catalog types, got %d", len(tipos))
	}

	// verify cuis/cufd in mem repos
	if _, err := cuisRepo.GetActiveByPos(pos.ID); err != nil {
		t.Fatalf("missing cuis: %v", err)
	}
	if _, err := cufdRepo.GetActiveByPos(pos.ID); err != nil {
		t.Fatalf("missing cufd: %v", err)
	}
	_ = posRepo
}

func TestProvision_RejectedBySiat_NotEnabledNoCufd(t *testing.T) {
	srv := mockSiatServer(t, false)
	defer srv.Close()

	tipoPVRepo := &memTipoPVRepo{m: map[string][]*domain.TipoPuntoVenta{}}
	svc, _, _, _, cuisRepo, cufdRepo := newProvisionService(srv.URL, tipoPVRepo)

	input := CreatePOSInput{Description: "desc", Name: "POS-1", CodigoTipoPuntoVenta: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pos, err := svc.CreateOperationalPOS(ctx, "branch-1", input)
	if err == nil {
		t.Fatalf("expected error when SIAT rejects registration")
	}
	if pos == nil {
		t.Fatalf("expected POS returned with ERROR status")
	}
	if pos.Status != "ERROR" {
		t.Fatalf("expected ERROR, got %s", pos.Status)
	}
	if pos.SiatError == nil || !strings.Contains(*pos.SiatError, "ERROR-1") {
		t.Fatalf("expected siat error recorded, got %v", pos.SiatError)
	}
	if pos.SiatCode != nil {
		t.Fatalf("expected no official code when rejected, got %v", *pos.SiatCode)
	}

	// CUFD must NOT be requested/persisted for a rejected POS
	if _, err := cufdRepo.GetActiveByPos(pos.ID); err == nil {
		t.Fatalf("expected no CUFD for rejected POS")
	}
	// CUIS must NOT be persisted either: the sucursal-level CUIS is transient and
	// the POS-scoped CUIS is only requested after SIAT confirms the registration.
	if _, err := cuisRepo.GetActiveByPos(pos.ID); err == nil {
		t.Fatalf("expected no CUIS for rejected POS")
	}
}

func TestProvision_InvalidTipoRejected(t *testing.T) {
	srv := mockSiatServer(t, true)
	defer srv.Close()

	tipoPVRepo := &memTipoPVRepo{m: map[string][]*domain.TipoPuntoVenta{}}
	svc, _, _, _, _, _ := newProvisionService(srv.URL, tipoPVRepo)

	input := CreatePOSInput{Description: "desc", Name: "POS-1", CodigoTipoPuntoVenta: 99}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pos, err := svc.CreateOperationalPOS(ctx, "branch-1", input)
	if err == nil {
		t.Fatalf("expected error for invalid tipo")
	}
	if !strings.Contains(err.Error(), "no válido") {
		t.Fatalf("expected invalid tipo error, got: %v", err)
	}
	if pos == nil || pos.Status != "ERROR" {
		t.Fatalf("expected POS ERROR status, got %v", pos)
	}
}
