package usecase

import (
	"github.com/brandsrx/supay/internal/domain"
	"strconv"
	"time"
)

type fakeInvoiceRepo struct {
	invoices    map[string]*domain.Invoice
	claimCalls  int
	updateCalls int
	updateErr   error
	activeCufd  *domain.Cufd
	lastFields  map[string]any
}

func (f *fakeInvoiceRepo) GetByIDs(tenantID string, ids []string) ([]*domain.Invoice, error) {
	var result []*domain.Invoice
	for _, id := range ids {
		if inv, exists := f.invoices[id]; exists && inv.CompanyId == tenantID {
			result = append(result, inv)
		}
	}
	return result, nil
}

func (f *fakeInvoiceRepo) ListByPointOfSale(tenantID, posID string) ([]*domain.Invoice, error) {
	out := make([]*domain.Invoice, 0)
	for _, inv := range f.invoices {
		if inv.CompanyId == tenantID && inv.PointOfSaleId == posID {
			out = append(out, inv)
		}
	}
	return out, nil
}

func (f *fakeInvoiceRepo) ReleaseStaleSending(d time.Duration) (int64, error) {
	return 0, nil
}

func newFakeInvoiceRepo() *fakeInvoiceRepo {
	return &fakeInvoiceRepo{invoices: map[string]*domain.Invoice{}}
}

func (f *fakeInvoiceRepo) Create(inv *domain.Invoice) error {
	if inv.ID == "" {
		inv.ID = "inv-" + strconv.Itoa(len(f.invoices)+1)
	}
	f.invoices[inv.ID] = inv
	return nil
}

func (f *fakeInvoiceRepo) GetByID(tenantID, id string) (*domain.Invoice, error) {
	inv, ok := f.invoices[id]
	if !ok || inv.CompanyId != tenantID {
		return nil, domain.ErrNotFound
	}
	return inv, nil
}

func (f *fakeInvoiceRepo) ListFiltered(filter domain.InvoiceListFilter) ([]*domain.Invoice, int64, error) {
	out := make([]*domain.Invoice, 0)
	for _, inv := range f.invoices {
		if inv.CompanyId != filter.TenantID || inv.PointOfSaleId != filter.PointOfSaleID {
			continue
		}
		if filter.Status != nil && inv.Status != *filter.Status {
			continue
		}
		out = append(out, inv)
	}
	total := int64(len(out))
	if filter.Offset < len(out) {
		out = out[filter.Offset:]
	} else {
		out = nil
	}
	if filter.Limit > 0 && len(out) > filter.Limit {
		out = out[:filter.Limit]
	}
	return out, total, nil
}

func (f *fakeInvoiceRepo) Update(inv *domain.Invoice) error {
	f.updateCalls++
	if f.updateErr != nil {
		return f.updateErr
	}
	f.invoices[inv.ID] = inv
	return nil
}

func (f *fakeInvoiceRepo) ClaimForEmission(tenantID, id string) (bool, error) {
	f.claimCalls++
	inv, ok := f.invoices[id]
	if !ok || inv.CompanyId != tenantID {
		return false, nil
	}
	if inv.Status != domain.InvoicePending {
		return false, nil
	}
	claimed := *inv
	claimed.Status = domain.InvoiceSending
	f.invoices[id] = &claimed
	return true, nil
}

func (f *fakeInvoiceRepo) TransitionStatus(tenantID, id string, from, to domain.InvoiceStatus, reason domain.InvoiceTransitionReason, fields map[string]any, event *domain.InvoiceEvent) (bool, error) {
	if err := (domain.InvoiceStateMachine{}).Transition(from, to, reason); err != nil {
		return false, err
	}
	inv, ok := f.invoices[id]
	if !ok || inv.CompanyId != tenantID {
		return false, nil
	}
	claimed, err := f.ClaimStatus(id, from, to, fields)
	if claimed {
		f.updateCalls++
	}
	return claimed, err
}

func (f *fakeInvoiceRepo) ClaimStatus(id string, from, to domain.InvoiceStatus, fields map[string]any) (bool, error) {
	inv, ok := f.invoices[id]
	if !ok || inv.Status != from {
		return false, nil
	}
	inv.Status = to
	f.lastFields = make(map[string]any, len(fields))
	for k, v := range fields {
		f.lastFields[k] = v
		switch k {
		case "motivo_anulacion":
			if m, ok := v.(int); ok {
				inv.MotivoAnulacion = &m
			} else if m, ok := v.(*int); ok {
				inv.MotivoAnulacion = m
			}
		case "fecha_anulacion":
			if t, ok := v.(time.Time); ok {
				inv.FechaAnulacion = &t
			} else if t, ok := v.(*time.Time); ok {
				inv.FechaAnulacion = t
			}
		case "siat_reception_code":
			if c, ok := v.(string); ok && c != "" {
				inv.SiatReceptionCode = &c
			}
		case "cuf":
			inv.Cuf, _ = v.(*string)
		case "xml":
			inv.Xml, _ = v.(*string)
		case "xml_hash":
			inv.XmlHash, _ = v.(*string)
		case "archivo":
			inv.Archivo, _ = v.(string)
		case "hash_archivo":
			inv.HashArchivo, _ = v.(string)
		case "contingency_event_id":
			if id, ok := v.(string); ok {
				inv.ContingencyEventId = &id
			}
		case "emission_type":
			inv.EmissionType, _ = v.(string)
		}
	}
	return true, nil
}

func (f *fakeInvoiceRepo) FindActiveCufdForPointOfSale(string, time.Time) (*domain.Cufd, error) {
	return f.activeCufd, nil
}
