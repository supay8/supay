package usecase

import (
	"testing"

	"github.com/brandsrx/supay/internal/domain"
	"gorm.io/gorm"
)

type phase12CompanyRepo struct {
	items map[string]*domain.Company
	err   error
}

func (r *phase12CompanyRepo) Create(value *domain.Company) error {
	if r.err != nil {
		return r.err
	}
	if value.ID == "" {
		value.ID = "company-created"
	}
	r.items[value.ID] = value
	return nil
}
func (r *phase12CompanyRepo) GetByNit(nit string) (*domain.Company, error) {
	if r.err != nil {
		return nil, r.err
	}
	for _, value := range r.items {
		if value.Nit == nit {
			return value, nil
		}
	}
	return nil, gorm.ErrRecordNotFound
}
func (r *phase12CompanyRepo) GetByID(id string) (*domain.Company, error) {
	if r.err != nil {
		return nil, r.err
	}
	if value := r.items[id]; value != nil {
		return value, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (r *phase12CompanyRepo) Update(value *domain.Company) error {
	if r.err != nil {
		return r.err
	}
	r.items[value.ID] = value
	return nil
}
func (r *phase12CompanyRepo) Delete(id string) error {
	if r.err != nil {
		return r.err
	}
	delete(r.items, id)
	return nil
}

type phase12BranchRepo struct{ item *domain.Branch }

func (r *phase12BranchRepo) Create(v *domain.Branch) error { v.ID = "branch-1"; r.item = v; return nil }
func (r *phase12BranchRepo) GetByID(id string) (*domain.Branch, error) {
	if r.item == nil || r.item.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return r.item, nil
}
func (r *phase12BranchRepo) GetByCompanyAndSucursal(companyID string, code int) (*domain.Branch, error) {
	if r.item != nil && r.item.CompanyID == companyID && r.item.CodigoSucursal == code {
		return r.item, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (r *phase12BranchRepo) List(string) ([]*domain.Branch, error) {
	if r.item == nil {
		return nil, nil
	}
	return []*domain.Branch{r.item}, nil
}
func (r *phase12BranchRepo) Update(v *domain.Branch) error { r.item = v; return nil }
func (r *phase12BranchRepo) Delete(string) error           { r.item = nil; return nil }

type phase12CustomerRepo struct{ item *domain.Customer }

func (r *phase12CustomerRepo) Create(v *domain.Customer) error {
	v.ID = "customer-1"
	r.item = v
	return nil
}
func (r *phase12CustomerRepo) GetByID(id string) (*domain.Customer, error) {
	if r.item == nil || r.item.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return r.item, nil
}
func (r *phase12CustomerRepo) GetByCompanyAndFiscalIdentity(c, dt, dn string, _ *string, name, _ string) (*domain.Customer, error) {
	if r.item != nil && r.item.CompanyId == c && r.item.DocumentType == dt && r.item.DocumentNumber == dn && r.item.Name == name {
		return r.item, nil
	}
	return nil, gorm.ErrRecordNotFound
}
func (r *phase12CustomerRepo) List(string) ([]*domain.Customer, error) {
	if r.item == nil {
		return nil, nil
	}
	return []*domain.Customer{r.item}, nil
}

type phase12POSRepo struct{ item *domain.PointOfSale }

func (r *phase12POSRepo) Create(v *domain.PointOfSale) error { v.ID = "pos-1"; r.item = v; return nil }
func (r *phase12POSRepo) GetByID(id string) (*domain.PointOfSale, error) {
	if r.item == nil || r.item.ID != id {
		return nil, gorm.ErrRecordNotFound
	}
	return r.item, nil
}
func (r *phase12POSRepo) List(string) ([]*domain.PointOfSale, error) {
	if r.item == nil {
		return nil, nil
	}
	return []*domain.PointOfSale{r.item}, nil
}
func (r *phase12POSRepo) ListByBranch(string) ([]*domain.PointOfSale, error) { return r.List("") }
func (r *phase12POSRepo) Update(v *domain.PointOfSale) error                 { r.item = v; return nil }
func (r *phase12POSRepo) Delete(string) error                                { r.item = nil; return nil }

func TestCompanyUsecaseLifecycleAndValidation(t *testing.T) {
	repo := &phase12CompanyRepo{items: map[string]*domain.Company{}}
	uc := NewCompanyUsecase(repo)
	for _, request := range []RegisterCompanyRequest{{}, {Nit: "1"}, {Nit: "1", BusinessName: "ACME", Ambiente: "INVALID"}, {Nit: "1", BusinessName: "ACME", CertificateWebhookURL: "ftp://bad"}} {
		if _, err := uc.Register(request); err == nil {
			t.Fatalf("se esperaba error para %+v", request)
		}
	}
	company, err := uc.Register(RegisterCompanyRequest{Nit: "123", BusinessName: " ACME ", CertificateWebhookURL: " https://example.com/hook "})
	if err != nil || company.Ambiente != domain.EnvironmentPiloto || company.UsuarioSiat != "SUPAY" {
		t.Fatalf("Register=%+v err=%v", company, err)
	}
	if _, err = uc.Register(RegisterCompanyRequest{Nit: "123", BusinessName: "duplicada"}); err == nil {
		t.Fatal("se esperaba conflicto NIT")
	}
	if got, err := uc.GetByNit("123"); err != nil || got.ID != company.ID {
		t.Fatalf("GetByNit=%+v err=%v", got, err)
	}
	name, system, municipality, address, phone, activity, footer, webhook := "Nueva", "SYS", "La Paz", "Calle", "700", "101010", "pie", "https://example.org/hook"
	env := domain.EnvironmentProduccion
	updated, err := uc.Update(UpdateCompanyRequest{BusinessName: &name, CodigoSistema: &system, Ambiente: &env, Municipio: &municipality, Direccion: &address, Telefono: &phone, CodigoActividad: &activity, PiePagina: &footer, CertificateWebhookURL: &webhook}, company.ID)
	if err != nil || updated.BusinessName != name || updated.Ambiente != env || updated.CertificateWebhookURL != webhook {
		t.Fatalf("Update=%+v err=%v", updated, err)
	}
	if err := uc.Delete(company.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := uc.GetByNit("missing"); err == nil {
		t.Fatal("se esperaba not found")
	}
}

func TestBranchCustomerAndPointOfSaleLifecycles(t *testing.T) {
	companies := &phase12CompanyRepo{items: map[string]*domain.Company{"company-1": {ID: "company-1"}}}
	branches := &phase12BranchRepo{}
	branchUC := NewBranchUsecase(branches, companies)
	for _, request := range []CreateBranchRequest{{}, {CompanyID: "company-1"}, {CompanyID: "company-1", Name: "Central", CodigoSucursal: -1}} {
		if _, err := branchUC.Create(request); err == nil {
			t.Fatalf("branch validation %+v", request)
		}
	}
	branch, err := branchUC.Create(CreateBranchRequest{CompanyID: "company-1", Name: "Central", Address: "Calle 1"})
	if err != nil {
		t.Fatal(err)
	}
	code, name, active := 2, "Sur", false
	if _, err = branchUC.Update(UpdateBranchRequest{CodigoSucursal: &code, Name: &name, Active: &active}, branch.ID); err != nil {
		t.Fatal(err)
	}
	if list, err := branchUC.List("company-1"); err != nil || len(list) != 1 {
		t.Fatalf("branches=%v err=%v", list, err)
	}
	if err = branchUC.Delete(branch.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = branchUC.GetByID("missing"); err == nil {
		t.Fatal("branch not found esperado")
	}

	customers := &phase12CustomerRepo{item: &domain.Customer{ID: "customer-1", CompanyId: "company-1", DocumentType: "CI", DocumentNumber: "123", Name: "Ada"}}
	customerUC := NewCustomerUsecase(customers)
	if list, _ := customerUC.List("company-1"); len(list) != 1 {
		t.Fatalf("customers=%v", list)
	}
	if _, err = customerUC.GetByID("missing"); err == nil {
		t.Fatal("customer not found esperado")
	}

	points := &phase12POSRepo{}
	posBranch := &phase12BranchRepo{item: &domain.Branch{ID: "branch-pos", CompanyID: "company-1", CodigoSucursal: 7, Active: true}}
	posUC := NewPointOfSaleUsecase(points, posBranch)
	if _, err := posUC.Register(RegisterPointOfSaleRequest{}); err == nil {
		t.Fatal("validación POS esperada")
	}
	cuis := "CUIS"
	pos, err := posUC.Register(RegisterPointOfSaleRequest{BranchId: "branch-pos", Name: "Caja", Description: "Caja", TipoPuntoVenta: 1, Cuis: &cuis})
	if err != nil || pos.CuisCreatedAt == nil || pos.BranchId != "branch-pos" || pos.CompanyId != "company-1" || pos.CodigoSucursal != 7 {
		t.Fatalf("pos=%+v err=%v", pos, err)
	}
	description, newCuis := "Caja 2", "CUIS-2"
	if _, err = posUC.Update(UpdatePointOfSaleRequest{Description: &description, Cuis: &newCuis}, pos.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := posUC.List("company-1"); len(list) != 1 {
		t.Fatalf("points=%v", list)
	}
	if err = posUC.Delete(pos.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = posUC.GetByID("missing"); err == nil {
		t.Fatal("pos not found esperado")
	}
}
