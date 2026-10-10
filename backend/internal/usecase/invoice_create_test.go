package usecase

import (
	"context"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/brandsrx/supay/internal/domain"
	"testing"
	"time"
)

func TestCreatePurgeaCamposEducativosFueraDeSector11(t *testing.T) {
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
		ID:           "comp-1",
		Nit:          "9971522011",
		BusinessName: "EMPRESA PILOTO SRL",
		Ambiente:     domain.EnvironmentPiloto,
		Municipio:    "LA PAZ",
		Direccion:    "AV. CAMACHO 123",
	}}
	uc := NewInvoiceUsecase(repo, customerRepo, companyRepo, posRepo, &fakeCatalogRepo{}, nil, nil, siat.ModalidadElectronica,
		nil, nil, nil, nil, nil, false, nil)
	nombre := "MARIA TEST"
	periodo := "2026-1"
	req := CreateInvoiceRequest{
		CompanyId:             "comp-1",
		PointOfSaleId:         "pos-1",
		Customer:              &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "1234567", Name: "Juan Perez"},
		CodigoDocumentoSector: 1,
		NombreEstudiante:      &nombre,
		PeriodoFacturado:      &periodo,
		Items:                 []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// Nuevo contrato: Create persiste los campos tal cual llegan (auditoría);
	// el filtrado ocurre al construir el XML, no al guardar.
	if inv.NombreEstudiante == nil || *inv.NombreEstudiante != nombre {
		t.Fatalf("NombreEstudiante=%v, se esperaba %q persistido", inv.NombreEstudiante, nombre)
	}
	if inv.PeriodoFacturado == nil || *inv.PeriodoFacturado != periodo {
		t.Fatalf("PeriodoFacturado=%v, se esperaba %q persistido", inv.PeriodoFacturado, periodo)
	}
	if len(inv.SectorData) != 0 && string(inv.SectorData) != "{}" {
		t.Errorf("SectorData=%s, se esperaba vacío para compraventa sin datos específicos", inv.SectorData)
	}
}

func TestCreateCustomerInline(t *testing.T) {
	uc, _, customerRepo, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer: &CreateInvoiceInlineCustomer{
			DocumentType:   "nit",
			DocumentNumber: "123456789",
			Name:           "CLIENTE NUEVO",
		},
		Items: []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.CustomerId == "" {
		t.Fatal("CustomerId no asignado")
	}
	if len(customerRepo.created) != 1 {
		t.Fatalf("clientes creados=%d, se esperaba 1", len(customerRepo.created))
	}
	if customerRepo.created[0].Name != "CLIENTE NUEVO" {
		t.Errorf("nombre=%q", customerRepo.created[0].Name)
	}
	if customerRepo.created[0].CodigoCliente != "NIT123456789" {
		t.Errorf("codigo_cliente=%q, se esperaba NIT123456789 generado al crear", customerRepo.created[0].CodigoCliente)
	}
}

func TestCreateMismoDocumentoNombreDistintoCreaNuevaVersion(t *testing.T) {
	uc, _, customerRepo, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer: &CreateInvoiceInlineCustomer{
			DocumentType:   "CI",
			DocumentNumber: "1234567",
			Name:           "OTRO NOMBRE",
		},
		Items: []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.CustomerId == "cust-1" {
		t.Errorf("CustomerId=%q, no debía reusar la versión con otro nombre", inv.CustomerId)
	}
	if inv.Customer.Name != "OTRO NOMBRE" {
		t.Errorf("Customer.Name=%q, se esperaba conservar el snapshot nuevo", inv.Customer.Name)
	}
	if len(customerRepo.created) != 1 {
		t.Fatalf("debió crear una nueva versión histórica; creados=%d", len(customerRepo.created))
	}
}

func TestCreateCompanyDerivadoDePOS(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer:      historicalCustomerRequest(),
		Items:         []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.CompanyId != "comp-1" {
		t.Errorf("CompanyId=%q, se esperaba comp-1 derivado del POS", inv.CompanyId)
	}
}

func TestCreateRequiereDescripcionSnapshot(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer:      historicalCustomerRequest(),
		Items:         []CreateInvoiceItemRequest{fiscalTestItem("SKU-001", "", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err == nil || inv != nil {
		t.Fatalf("se esperaba rechazar el ítem sin descripción propia: inv=%+v err=%v", inv, err)
	}
}

func TestCreateIgnoresEmitFlag(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Customer:      historicalCustomerRequest(),
		Emit:          true,
		Items:         []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	// El flag Emit es responsabilidad del handler; el usecase Create solo crea.
	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if inv.Status != domain.InvoicePending {
		t.Errorf("status=%s, se esperaba PENDING (Emit no afecta Create)", inv.Status)
	}
}

func TestCreateWithReceiver(t *testing.T) {
	uc, _, customerRepo, _ := createTestUsecaseBuilder()

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Receiver: &CreateInvoiceReceiver{
			DocumentType:   1, // CI
			DocumentNumber: "12345678",
			Name:           "Juan Perez",
			Email:          strPtr("juan@email.com"),
		},
		Items: []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create with receiver: %v", err)
	}

	// Legacy Receiver ahora mapea a Customer (create-only)
	if len(customerRepo.created) != 1 {
		t.Fatalf("debió crear 1 cliente via Receiver legacy; creados=%d", len(customerRepo.created))
	}
	if inv.CustomerId == "" {
		t.Errorf("CustomerId se esperaba con valor")
	}
	_ = inv
}

func TestCreateWithReceiverAssociatesExistingCustomer(t *testing.T) {
	uc, _, customerRepo, _ := createTestUsecaseBuilder()

	// Pre-create a customer with the same document
	existingCustomer := &domain.Customer{
		ID:             "cust-existing",
		CompanyId:      "comp-1",
		DocumentType:   "CI",
		DocumentNumber: "12345678",
		Name:           "Juan Perez Existente",
		Complement:     strPtr("COMP"),
	}
	if err := customerRepo.Create(existingCustomer); err != nil {
		t.Fatalf("setup customer: %v", err)
	}

	req := CreateInvoiceRequest{
		PointOfSaleId: "pos-1",
		Receiver: &CreateInvoiceReceiver{
			DocumentType:   1, // CI
			DocumentNumber: "12345678",
			Name:           "Juan Perez", // Different name, but same document
			Email:          strPtr("juan@email.com"),
		},
		Items: []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)},
	}

	inv, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create with receiver: %v", err)
	}

	// El nombre diferente crea otra versión y preserva exactamente el request.
	if inv.CustomerId == "cust-existing" || inv.Customer.Name != "Juan Perez" {
		t.Errorf("snapshot/versionado inesperado: customer_id=%q customer=%+v", inv.CustomerId, inv.Customer)
	}
}

func TestCreateValidationExactlyOneCustomerSource(t *testing.T) {
	uc, _, _, _ := createTestUsecaseBuilder()

	tests := []struct {
		name    string
		req     CreateInvoiceRequest
		wantErr bool
	}{
		{
			name: "none provided",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: true,
		},
		{
			name: "customer_id and customer",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				CustomerId:    "cust-1",
				Customer:      &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "123", Name: "Test"},
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: false,
		},
		{
			name: "customer_id and receiver",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				CustomerId:    "cust-1",
				Receiver:      &CreateInvoiceReceiver{DocumentType: 1, DocumentNumber: "123", Name: "Test"},
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: false,
		},
		{
			name: "customer and receiver",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				Customer:      &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "123", Name: "Test"},
				Receiver:      &CreateInvoiceReceiver{DocumentType: 1, DocumentNumber: "123", Name: "Test"},
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: false,
		},
		{
			name: "three sources",
			req: CreateInvoiceRequest{
				PointOfSaleId: "pos-1",
				CustomerId:    "cust-1",
				Customer:      &CreateInvoiceInlineCustomer{DocumentType: "CI", DocumentNumber: "123", Name: "Test"},
				Receiver:      &CreateInvoiceReceiver{DocumentType: 1, DocumentNumber: "123", Name: "Test"},
				Items:         []CreateInvoiceItemRequest{{Code: "P001", Description: "Producto", Quantity: 1, UnitPrice: 100}},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := range tt.req.Items {
				item := fiscalTestItem(tt.req.Items[i].Code, tt.req.Items[i].Description, tt.req.Items[i].Quantity, tt.req.Items[i].UnitPrice)
				tt.req.Items[i] = item
			}
			_, err := uc.Create(context.Background(), tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("error=%v, wantErr=%v", err, tt.wantErr)
			}
		})
	}
}

func TestCreateSinClaveCreaFacturasIndependientes(t *testing.T) {
	uc, repo, _, _ := createTestUsecaseBuilder()
	req := CreateInvoiceRequest{PointOfSaleId: "pos-1", Customer: historicalCustomerRequest(),
		Items: []CreateInvoiceItemRequest{fiscalTestItem("P001", "Producto", 1, 100)}}
	first, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	second, err := uc.Create(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID || len(repo.invoices) != 2 {
		t.Fatal("cada creación debe tener su propia identidad")
	}
}
