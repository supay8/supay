package siat

import (
	"encoding/xml"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFacadeRoutingIgnoresHistoricalField(t *testing.T) {
	profile, err := PerfilSector(SectorCompraVenta)
	if err != nil {
		t.Fatal(err)
	}
	historical := profile.Fachada
	profile.Fachada = FachadaPorModalidad
	t.Cleanup(func() { profile.Fachada = historical })
	if profile.Facade.Fixed() != FachadaCompraVenta || profile.Facade.IsByModalidad() {
		t.Fatalf("el selector vigente fue afectado por metadata histórica: %+v", profile.Facade)
	}
}

func TestSector33PaqueteUsaArchivoSinConstruirHijos(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<Envelope><Body><recepcionPaqueteFacturaResponse><RespuestaServicioFacturacion><transaccion>true</transaccion><codigoEstado>908</codigoEstado><codigoRecepcion>RCV-33-P</codigoRecepcion></RespuestaServicioFacturacion></recepcionPaqueteFacturaResponse></Body></Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)
	_, err := svc.EnviarPaqueteFactura(t.Context(), SolicitudPaqueteFactura{
		CodigoAmbiente: AmbientePruebas, CodigoSistema: "SYS-123", Nit: "1020304050",
		Modalidad: ModalidadElectronica, Cuis: "CUIS", Cufd: "CUFD", CodigoControl: "CONTROL",
		CodigoDocumentoSector: 33, CodigoEvento: 1, Archivo: "ARCHIVO-33", HashArchivo: "HASH-33",
		Facturas: []SolicitudFactura{{}},
	})
	if err != nil {
		t.Fatalf("sector 33 paquete: %v", err)
	}
	for _, want := range []string{"<archivo>ARCHIVO-33</archivo>", "<hashArchivo>HASH-33</hashArchivo>", "<cantidadFacturas>1</cantidadFacturas>"} {
		if !strings.Contains(body, want) {
			t.Errorf("payload sector 33 no contiene %s: %s", want, body)
		}
	}
}

func TestSector33MasivaUsaArchivoSinConstruirHijos(t *testing.T) {
	var body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		data, _ := io.ReadAll(r.Body)
		body = string(data)
		w.Header().Set("Content-Type", "application/xml")
		_, _ = w.Write([]byte(`<Envelope><Body><recepcionMasivaFacturaResponse><RespuestaServicioFacturacion><transaccion>true</transaccion><codigoEstado>908</codigoEstado><codigoRecepcion>RCV-33-M</codigoRecepcion></RespuestaServicioFacturacion></recepcionMasivaFacturaResponse></Body></Envelope>`))
	}))
	defer server.Close()

	svc := newTestService(t, server.URL)
	_, err := svc.EnviarMasivaFacturas(t.Context(), SolicitudMasivaFactura{
		CodigoAmbiente: AmbientePruebas, CodigoSistema: "SYS-123", Nit: "1020304050",
		Modalidad: ModalidadComputarizada, Cuis: "CUIS", Cufd: "CUFD", CodigoControl: "CONTROL",
		CodigoDocumentoSector: 33, Archivo: "ARCHIVO-33-M", HashArchivo: "HASH-33-M",
		Facturas: []SolicitudFactura{{}},
	})
	if err != nil {
		t.Fatalf("sector 33 masiva: %v", err)
	}
	for _, want := range []string{"<archivo>ARCHIVO-33-M</archivo>", "<hashArchivo>HASH-33-M</hashArchivo>", "<cantidadFacturas>1</cantidadFacturas>"} {
		if !strings.Contains(body, want) {
			t.Errorf("payload sector 33 no contiene %s: %s", want, body)
		}
	}
}

func TestCodigoDocumentoSectorCabeceraYSolicitud(t *testing.T) {
	correcto := codigoDocumentoSectorFixture{Codigo: SectorCompraVenta}
	if err := validarCodigoDocumentoSectorFactura(SectorCompraVenta, SectorCompraVenta, correcto); err != nil {
		t.Fatalf("código coincidente rechazado: %v", err)
	}
	if err := validarCodigoDocumentoSectorFactura(SectorCompraVenta, SectorCompraVenta, codigoDocumentoSectorFixture{Codigo: SectorEducativo}); err == nil {
		t.Fatal("se esperaba rechazo de código de cabecera discrepante")
	}
	if err := validarCodigoDocumentoSectorFactura(SectorEducativo, SectorCompraVenta, correcto); err == nil {
		t.Fatal("se esperaba rechazo de código de solicitud discrepante")
	}
}

type codigoDocumentoSectorFixture struct {
	XMLName xml.Name `xml:"factura"`
	Codigo  int      `xml:"codigoDocumentoSector"`
}

func TestRegistroValidaMetadatosDeBuilders(t *testing.T) {
	for _, profile := range PerfilesSector() {
		if !profile.HasBuilder() {
			continue
		}
		cabecera := profile.builders.cabecera()
		for _, field := range profile.Campos {
			if field.Metodo != "" && !tieneMetodo(cabecera, field.Metodo) {
				t.Errorf("sector %d: campo %s declara método inexistente %s", profile.Codigo, field.JSON, field.Metodo)
			}
		}
		if profile.builders.detalle != nil {
			detalle := profile.builders.detalle()
			for _, field := range profile.CamposDetalle {
				if field.Metodo != "" && !tieneMetodo(detalle, field.Metodo) {
					t.Errorf("sector %d: campo detalle %s declara método inexistente %s", profile.Codigo, field.JSON, field.Metodo)
				}
			}
		}
		modalidades := profile.Modalidades
		if len(modalidades) == 0 {
			modalidades = []int{ModalidadElectronica, ModalidadComputarizada}
		}
		for _, modalidad := range modalidades {
			if profile.builders.factura(modalidad) == nil {
				t.Errorf("sector %d: constructor de factura vacío para modalidad %d", profile.Codigo, modalidad)
			}
		}
	}
}
