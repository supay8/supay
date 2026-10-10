package usecase

import (
	"context"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/ports"
	"github.com/brandsrx/supay/internal/storage"
	"time"
)

func invoiceContext() context.Context {
	return invoiceContextFor("comp-1")
}

func invoiceContextFor(tenantID string) context.Context {
	return siat.WithCompanyID(context.Background(), tenantID)
}

func strPtr(s string) *string { return &s }

func intPtr(i int) *int { return &i }

func testInvoice() *domain.Invoice {
	now := time.Now()
	actividad := "101010"
	sin := "5113100"
	unit := 58
	customerName := "Juan Perez"
	return &domain.Invoice{
		ID:               "inv-1",
		CompanyId:        "comp-1",
		CustomerId:       "cust-1",
		PointOfSaleId:    "pos-1",
		CufdId:           "cufd-1",
		InvoiceNumber:    1,
		EmissionType:     "EN_LINEA",
		CodigoMetodoPago: 1,
		CodigoMoneda:     1,
		TipoCambio:       1,
		IssueDate:        now,
		Total:            200,
		Status:           domain.InvoicePending,
		Company: domain.Company{
			ID:              "comp-1",
			Nit:             "9971522011",
			BusinessName:    "EMPRESA PILOTO SRL",
			Ambiente:        domain.EnvironmentPiloto,
			Municipio:       "LA PAZ",
			Direccion:       "AV. CAMACHO 123",
			Telefono:        "2444444",
			CodigoActividad: &actividad,
		},
		Customer: domain.Customer{
			ID:             "cust-1",
			DocumentType:   "CI",
			DocumentNumber: "1234567",
			Name:           customerName,
			CodigoCliente:  "CI1234567",
		},
		PointOfSale: domain.PointOfSale{
			ID:               "pos-1",
			CompanyId:        "comp-1",
			CodigoSucursal:   0,
			CodigoPuntoVenta: 3,
			Cuis:             strPtr("D17EEF19"),
			IsActive:         true,
		},
		CufdRecord: domain.Cufd{
			ID:          "cufd-1",
			Cufd:        "CUFD-XYZ",
			ControlCode: "CC-123",
			ValidFrom:   now.Add(-time.Hour),
			ValidTo:     now.Add(time.Hour),
			Active:      true,
		},
		Items: []domain.InvoiceItem{
			{
				ID:                "item-1",
				Code:              "P001",
				Description:       "Producto de prueba",
				CodigoProductoSin: &sin,
				UnitCode:          &unit,
				Quantity:          2,
				UnitPrice:         100,
				Subtotal:          200,
			},
		},
	}
}

func newTestUsecase(repo *fakeInvoiceRepo, catalog *fakeCatalogRepo, svc ports.FiscalService) *InvoiceUsecase {
	uc := NewInvoiceUsecase(repo, nil, nil, nil, catalog, nil, svc, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)
	uc.SetFileService(NewInvoiceFileService(
		storage.NewMemoryObjectStorage(),
		&fakeInvoiceFileRepo{invoices: repo, rows: map[string]*domain.InvoiceFile{}},
		time.Minute,
	))
	return uc
}

func emittedInvoice() *domain.Invoice {
	inv := testInvoice()
	cuf := "CUF-EMITIDO"
	inv.Cuf = &cuf
	inv.Status = domain.InvoiceAccepted
	return inv
}

// createTestUsecaseBuilder arma un InvoiceUsecase listo para tests de Create.
func createTestUsecaseBuilder() (*InvoiceUsecase, *fakeInvoiceRepo, *fakeCustomerRepo, struct{}) {
	repo := newFakeInvoiceRepo()
	repo.activeCufd = &domain.Cufd{
		ID:          "cufd-1",
		Cufd:        "CUFD-XYZ",
		ControlCode: "CC-123",
		ValidFrom:   time.Now().Add(-time.Hour),
		ValidTo:     time.Now().Add(time.Hour),
		Active:      true,
	}
	posRepo := &fakePointOfSaleRepo{pos: domain.PointOfSale{
		ID:               "pos-1",
		CompanyId:        "comp-1",
		CodigoSucursal:   0,
		CodigoPuntoVenta: 3,
		Cuis:             strPtr("D17EEF19"),
		IsActive:         true,
	}}
	customerRepo := newFakeCustomerRepo(domain.Customer{
		ID:             "cust-1",
		CompanyId:      "comp-1",
		DocumentType:   "CI",
		DocumentNumber: "1234567",
		Name:           "Juan Perez",
	})
	companyRepo := &fakeCompanyRepo{company: domain.Company{
		ID:              "comp-1",
		Nit:             "9971522011",
		BusinessName:    "EMPRESA PILOTO SRL",
		Ambiente:        domain.EnvironmentPiloto,
		Municipio:       "LA PAZ",
		Direccion:       "AV. CAMACHO 123",
		CodigoActividad: strPtr("101010"),
	}}
	docSectorRepo := &fakeDocSectorRepo{items: []*domain.SiatActividadDocSector{
		{CodigoActividad: "101010", CodigoDocumentoSector: siat.SectorCompraVenta, TipoDocumentoSector: "FCV"},
	}}
	uc := NewInvoiceUsecase(repo, customerRepo, companyRepo, posRepo, &fakeCatalogRepo{}, nil, nil, siat.ModalidadElectronica,
		nil, nil, docSectorRepo, nil, nil, false, nil)
	return uc, repo, customerRepo, struct{}{}
}

func fiscalTestItem(code, description string, quantity, unitPrice float64) CreateInvoiceItemRequest {
	activity, sinCode, unit := "101010", "5113100", 58
	return CreateInvoiceItemRequest{
		Code: code, Description: description, CodigoActividad: &activity,
		CodigoProductoSin: &sinCode, UnitCode: &unit,
		Quantity: quantity, UnitPrice: unitPrice,
	}
}

func historicalCustomerRequest() *CreateInvoiceInlineCustomer {
	return &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "1234567", Name: "Juan Perez"}
}

// toSiatSolicitudFactura convierte un FiscalDocument a SolicitudFactura para
// poder reutilizar helpers de perfil en tests de buildSolicitudFactura.
func toSiatSolicitudFactura(f ports.FiscalDocument) siat.SolicitudFactura {
	return siat.SolicitudFactura{
		CodigoAmbiente:        f.CodigoAmbiente,
		CodigoSistema:         f.CodigoSistema,
		Nit:                   f.Nit,
		Modalidad:             f.Modalidad,
		NumeroFactura:         f.NumeroFactura,
		NumeroFacturaOriginal: f.NumeroFacturaOriginal,
		CodigoSucursal:        f.CodigoSucursal,
		CodigoPuntoVenta:      f.CodigoPuntoVenta,
		Cuis:                  f.Cuis,
		Cufd:                  f.Cufd,
		CodigoControl:         f.CodigoControl,
		FechaEmision:          f.FechaEmision,
		Usuario:               f.Usuario,
		Leyenda:               f.Leyenda,
		RazonSocialEmisor:     f.RazonSocialEmisor,
		Municipio:             f.Municipio,
		Direccion:             f.Direccion,
		Telefono:              f.Telefono,
		CodigoMetodoPago:      f.CodigoMetodoPago,
		CodigoMoneda:          f.CodigoMoneda,
		TipoCambio:            f.TipoCambio,
		MontoTotal:            f.MontoTotal,
		CodigoDocumentoSector: f.CodigoDocumentoSector,
		Layout:                f.Layout,
		CodigoTipoFactura:     f.CodigoTipoFactura,
		Cafc:                  f.Cafc,
		DatosSector:           f.DatosSector,
		NombreEstudiante:      f.NombreEstudiante,
		PeriodoFacturado:      f.PeriodoFacturado,
		Archivo:               f.Archivo,
		HashArchivo:           f.HashArchivo,
		Cuf:                   f.Cuf,
	}
}
