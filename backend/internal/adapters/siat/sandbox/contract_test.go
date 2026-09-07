package sandbox

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/brandsrx/supay/internal/ports"
)

// Este test es el contrato ejecutable de ports.FiscalService. Si el puerto
// cambia, el sandbox de CI debe seguir devolviendo resultados coherentes.
func TestFiscalServiceContract(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	svc := NewFiscalService()

	emitted, err := svc.Emit(ctx, ports.FiscalDocument{})
	if err != nil || !emitted.Transaccion || emitted.CodigoEstado != 908 || emitted.Cuf == "" || emitted.CodigoRecepcion == "" {
		t.Fatalf("Emit no cumple contrato: %+v err=%v", emitted, err)
	}
	query := ports.FiscalDocumentQuery{Cuf: emitted.Cuf}
	for name, call := range map[string]func() (ports.FiscalDocumentResult, error){
		"verify": func() (ports.FiscalDocumentResult, error) { return svc.VerifyStatus(ctx, query) },
		"annul":  func() (ports.FiscalDocumentResult, error) { return svc.Annul(ctx, query, 1) },
		"revert": func() (ports.FiscalDocumentResult, error) { return svc.RevertAnnul(ctx, query) },
	} {
		result, err := call()
		if err != nil || !result.Transaccion || result.CodigoEstado == 0 || result.CodigoRecepcion == "" {
			t.Fatalf("%s no cumple contrato: %+v err=%v", name, result, err)
		}
	}

	cuis, err := svc.RequestCUIS(ctx, ports.CredentialRequest{})
	if err != nil || !cuis.Transaccion || !strings.HasPrefix(cuis.Codigo, "FAKE-CUIS-") || cuis.FechaVigencia.IsZero() {
		t.Fatalf("CUIS inválido: %+v err=%v", cuis, err)
	}
	cufd, err := svc.RequestCUFD(ctx, ports.CredentialRequest{Cuis: cuis.Codigo})
	if err != nil || !cufd.Transaccion || cufd.CodigoControl == "" || cufd.Direccion == "" || cufd.FechaVigencia.IsZero() {
		t.Fatalf("CUFD inválido: %+v err=%v", cufd, err)
	}
	event, err := svc.RegisterSignificantEvent(ctx, ports.FiscalEvent{})
	if err != nil || !event.Transaccion || event.CodigoRecepcion == "" {
		t.Fatalf("evento inválido: %+v err=%v", event, err)
	}

	docs := []ports.FiscalDocument{{}, {}}
	packageCalls := map[string]func() (ports.FiscalPackageResult, error){
		"send package":     func() (ports.FiscalPackageResult, error) { return svc.SendPackage(ctx, ports.FiscalPackage{Facturas: docs}) },
		"validate package": func() (ports.FiscalPackageResult, error) { return svc.ValidatePackage(ctx, ports.FiscalPackage{Facturas: docs}, "rcp") },
		"send bulk":        func() (ports.FiscalPackageResult, error) { return svc.SendBulk(ctx, ports.FiscalBulk{Facturas: docs}) },
		"validate bulk":    func() (ports.FiscalPackageResult, error) { return svc.ValidateBulk(ctx, ports.FiscalBulk{Facturas: docs}, "rcp") },
	}
	for name, call := range packageCalls {
		result, err := call()
		if err != nil || !result.Transaccion || result.CantidadFacturas != 2 || result.CodigoRecepcion == "" {
			t.Fatalf("%s no cumple contrato: %+v err=%v", name, result, err)
		}
	}
	purchase, err := svc.SendPurchases(ctx, ports.FiscalPurchase{})
	if err != nil || !purchase.Transaccion || purchase.CodigoRecepcion == "" {
		t.Fatalf("compras inválidas: %+v err=%v", purchase, err)
	}
	signed, err := svc.SignXML(ctx, ports.FiscalSignRequest{Xml: "<invoice/>"})
	if err != nil || !strings.Contains(signed.XmlFirmado, "<invoice/>") || signed.HashArchivo == "" {
		t.Fatalf("firma inválida: %+v err=%v", signed, err)
	}
	adjustment, err := svc.EmitAdjustment(ctx, ports.FiscalAdjustment{})
	if err != nil || !adjustment.Transaccion || adjustment.Cuf == "" || adjustment.CodigoRecepcion == "" {
		t.Fatalf("ajuste inválido: %+v err=%v", adjustment, err)
	}

	for _, operation := range ports.FiscalSyncOperations {
		result, err := svc.Synchronize(ctx, ports.FiscalSyncRequest{}, operation)
		if err != nil || !result.Transaccion {
			t.Fatalf("sincronización %s inválida: %+v err=%v", operation, result, err)
		}
	}
}

func TestFiscalServiceConcurrentCodesAreUnique(t *testing.T) {
	t.Parallel()
	const workers = 64
	svc := NewFiscalService()
	codes := make(chan string, workers)
	var wg sync.WaitGroup
	for range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := svc.Emit(context.Background(), ports.FiscalDocument{})
			if err != nil {
				t.Errorf("Emit: %v", err)
				return
			}
			codes <- result.CodigoRecepcion
		}()
	}
	wg.Wait()
	close(codes)
	seen := make(map[string]struct{}, workers)
	for code := range codes {
		if _, exists := seen[code]; exists {
			t.Fatalf("código duplicado bajo concurrencia: %s", code)
		}
		seen[code] = struct{}{}
	}
}
