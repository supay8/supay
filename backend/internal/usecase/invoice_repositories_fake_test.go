package usecase

import (
	"github.com/brandsrx/supay/internal/domain"
	"strconv"
	"time"
)

type fakeCatalogRepo struct {
	items map[string][]*domain.CatalogItem
}

func (f *fakeCatalogRepo) Replace(string, string, []domain.CatalogItem, time.Time) error {
	return nil
}

func (f *fakeCatalogRepo) List(_ string, tipo string) ([]*domain.CatalogItem, error) {
	return f.items[tipo], nil
}

func (f *fakeCatalogRepo) ListAll(string) (map[string][]*domain.CatalogItem, error) {
	return f.items, nil
}

type fakeCompanyRepo struct {
	company domain.Company
}

func (f *fakeCompanyRepo) Create(*domain.Company) error { return nil }

func (f *fakeCompanyRepo) GetByNit(string) (*domain.Company, error) { return &f.company, nil }

func (f *fakeCompanyRepo) GetByID(string) (*domain.Company, error) { return &f.company, nil }

func (f *fakeCompanyRepo) Update(*domain.Company) error { return nil }

func (f *fakeCompanyRepo) Delete(string) error { return nil }

type fakeCustomerRepo struct {
	byID       map[string]*domain.Customer
	byDocument map[string]*domain.Customer
	created    []*domain.Customer
}

func newFakeCustomerRepo(customers ...domain.Customer) *fakeCustomerRepo {
	f := &fakeCustomerRepo{
		byID:       map[string]*domain.Customer{},
		byDocument: map[string]*domain.Customer{},
	}
	for i := range customers {
		c := &customers[i]
		f.byID[c.ID] = c
		f.byDocument[documentKey(c.DocumentType, c.DocumentNumber)] = c
	}
	return f
}

func documentKey(documentType, documentNumber string) string {
	return documentType + "|" + documentNumber
}

func (f *fakeCustomerRepo) Create(c *domain.Customer) error {
	if c.ID == "" {
		c.ID = "cust-" + strconv.Itoa(len(f.byID)+1)
	}
	f.created = append(f.created, c)
	f.byID[c.ID] = c
	f.byDocument[documentKey(c.DocumentType, c.DocumentNumber)] = c
	return nil
}

func (f *fakeCustomerRepo) GetByID(id string) (*domain.Customer, error) {
	c, ok := f.byID[id]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeCustomerRepo) GetByCompanyAndDocument(_, documentType, documentNumber string) (*domain.Customer, error) {
	c, ok := f.byDocument[documentKey(documentType, documentNumber)]
	if !ok {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeCustomerRepo) GetByCompanyAndFiscalIdentity(companyID, documentType, documentNumber string, complement *string, name string, email string) (*domain.Customer, error) {
	c, err := f.GetByCompanyAndDocument(companyID, documentType, documentNumber)
	if err != nil || c.Name != name || cadenaOpcional(c.Complement) != cadenaOpcional(complement) {
		return nil, domain.ErrNotFound
	}
	return c, nil
}

func (f *fakeCustomerRepo) List(string) ([]*domain.Customer, error) { return nil, nil }

type fakePointOfSaleRepo struct {
	pos domain.PointOfSale
}

func (f *fakePointOfSaleRepo) Create(*domain.PointOfSale) error { return nil }

func (f *fakePointOfSaleRepo) GetByID(string) (*domain.PointOfSale, error) {
	return &f.pos, nil
}

func (f *fakePointOfSaleRepo) List(string) ([]*domain.PointOfSale, error) { return nil, nil }

func (f *fakePointOfSaleRepo) ListByBranch(string) ([]*domain.PointOfSale, error) {
	return nil, nil
}

func (f *fakePointOfSaleRepo) Update(*domain.PointOfSale) error { return nil }

func (f *fakePointOfSaleRepo) Delete(string) error { return nil }

type fakeCufdRepo struct {
	vigente *domain.Cufd
	err     error
}

func (f *fakeCufdRepo) Create(_ *domain.Cufd) error { return nil }

func (f *fakeCufdRepo) GetActiveByPos(_ string) (*domain.Cufd, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vigente, nil
}

func (f *fakeCufdRepo) GetByPosAndWindow(_ string, _, _ time.Time) (*domain.Cufd, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.vigente, nil
}

func (f *fakeCufdRepo) DeactivateExpired() error { return nil }

type fakeDocSectorRepo struct {
	items []*domain.SiatActividadDocSector
}

func (f *fakeDocSectorRepo) Replace(string, []domain.SiatActividadDocSector, time.Time) error {
	return nil
}

func (f *fakeDocSectorRepo) List(string) ([]*domain.SiatActividadDocSector, error) {
	return f.items, nil
}

func (f *fakeDocSectorRepo) ListByActividad(_ string, codigoActividad string) ([]*domain.SiatActividadDocSector, error) {
	out := make([]*domain.SiatActividadDocSector, 0)
	for _, item := range f.items {
		if item.CodigoActividad == codigoActividad {
			out = append(out, item)
		}
	}
	return out, nil
}

type fakeLeyendaRepo struct {
	items []*domain.SiatLeyenda
}

func (f *fakeLeyendaRepo) Replace(string, []domain.SiatLeyenda, time.Time) error {
	return nil
}

func (f *fakeLeyendaRepo) List(string) ([]*domain.SiatLeyenda, error) {
	return f.items, nil
}

func (f *fakeLeyendaRepo) ListByActividad(_ string, codigoActividad string) ([]*domain.SiatLeyenda, error) {
	out := make([]*domain.SiatLeyenda, 0)
	for _, item := range f.items {
		if item.CodigoActividad == codigoActividad {
			out = append(out, item)
		}
	}
	return out, nil
}
