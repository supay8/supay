package siat

import (
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	goSiat "github.com/ron86i/go-siat/v2"
)

type sdkSectorCase struct {
	code         int
	name         string
	electronic   string
	computarized string
}

// sdkCatalogCases reads the SDK's test catalog instead of duplicating its
// list locally. A new SDK builder therefore makes this test fail until it is
// explicitly represented by our registry.
func sdkCatalogCases(t *testing.T) []sdkSectorCase {
	t.Helper()
	cmd := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", "github.com/ron86i/go-siat/v2")
	modCacheBytes, err := cmd.Output()
	if err != nil {
		t.Fatalf("localizar módulo go-siat: %v", err)
	}
	path := filepath.Join(strings.TrimSpace(string(modCacheBytes)), "pkg", "models", "invoices", "sectores_catalogo_test.go")
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("leer catálogo fuente del SDK %s: %v", path, err)
	}
	re := regexp.MustCompile(`\{(\d+),\s*"([^"]+)",\s*"([^"]+)",\s*"([^"]+)"`)
	matches := re.FindAllStringSubmatch(string(source), -1)
	if len(matches) == 0 {
		t.Fatalf("no se encontraron casos en %s; el formato del catálogo cambió", path)
	}
	cases := make([]sdkSectorCase, 0, len(matches))
	for _, match := range matches {
		code, err := strconv.Atoi(match[1])
		if err != nil {
			t.Fatalf("código inválido en catálogo SDK: %v", err)
		}
		cases = append(cases, sdkSectorCase{code: code, name: match[2], electronic: match[3], computarized: match[4]})
	}
	return cases
}

func TestSectorRegistryParityWithSDKCatalog(t *testing.T) {
	cases := sdkCatalogCases(t)
	local := make(map[int][]*SectorProfile)
	for _, profile := range PerfilesSector() {
		if profile.HasBuilder() {
			local[profile.Codigo] = append(local[profile.Codigo], profile)
		}
	}
	if got := countBuilderProfiles(local); got != len(cases) {
		t.Fatalf("builders locales=%d, casos builder SDK=%d; el registro quedó desfasado", got, len(cases))
	}

	seen := make(map[int]int)
	for _, sdkCase := range cases {
		profiles := local[sdkCase.code]
		if len(profiles) == 0 {
			t.Errorf("SDK agregó o documenta sector %d (%s) sin entrada local", sdkCase.code, sdkCase.name)
			continue
		}
		index := seen[sdkCase.code]
		if index >= len(profiles) {
			t.Errorf("sector %d tiene más variantes en SDK que en el registro local", sdkCase.code)
			continue
		}
		profile := profiles[index]
		seen[sdkCase.code]++
		if sdkCase.code == SectorNotaCreditoDebito {
			wantLayout := string(LayoutNotaCreditoDebito)
			if strings.Contains(strings.ToLower(sdkCase.name), "fiscal") {
				wantLayout = string(LayoutNotaFiscalCreditoDebito)
			}
			if profile.Layout != wantLayout {
				t.Errorf("sector 24 (%s): layout local=%q, SDK=%q", sdkCase.name, profile.Layout, wantLayout)
			}
		}
		assertSDKRoots(t, profile, sdkCase)
	}

	if profile, err := PerfilSector(33); err != nil || profile.HasBuilder() {
		t.Fatalf("sector 33 debe estar registrado explícitamente sin builder: profile=%v err=%v", profile, err)
	}
	sector52, err := PerfilSectorLayout(52, "")
	if err != nil {
		t.Fatal(err)
	}
	if err := sector52.ValidarModalidad(ModalidadComputarizada); err == nil {
		t.Fatal("sector 52 debe rechazar modalidad computarizada")
	}
}

func countBuilderProfiles(profiles map[int][]*SectorProfile) int {
	count := 0
	for _, entries := range profiles {
		count += len(entries)
	}
	return count
}

func assertSDKRoots(t *testing.T, profile *SectorProfile, sdkCase sdkSectorCase) {
	t.Helper()
	if !profile.Soportado {
		t.Fatalf("constructor del SDK sin soporte local: %s", sdkCase)
	}
	for _, mode := range []struct {
		code int
		root string
	}{
		{ModalidadElectronica, sdkCase.electronic},
		{ModalidadComputarizada, sdkCase.computarized},
	} {
		if err := profile.ValidarModalidad(mode.code); err != nil {
			if profile.Codigo == 52 && mode.code == ModalidadComputarizada {
				continue
			}
			t.Fatalf("sector %d modalidad %d no permitida inesperadamente: %v", profile.Codigo, mode.code, err)
		}
		req := SolicitudFactura{
			CodigoAmbiente: AmbientePruebas, CodigoSistema: "SYS-TEST", Nit: "1020304050",
			Modalidad: mode.code, NumeroFactura: 1, Cuis: "CUIS-TEST", Cufd: "CUFD-TEST",
			CodigoControl: "CONTROL-CODE-29-CHARACTERS-01", FechaEmision: time.Date(2025, 8, 15, 10, 30, 0, 0, LaPaz),
			Usuario: "SUPAY", Leyenda: "Ley 453", RazonSocialEmisor: "EMPRESA TEST SRL",
			Municipio: "LA PAZ", Direccion: "AV. TEST 123", CodigoMetodoPago: 1, CodigoMoneda: 1,
			TipoCambio: 1, MontoTotal: 100, CodigoDocumentoSector: profile.Codigo, Layout: profile.Layout,
			DatosSector: jsonRaw(datosDeEjemplo(profile)), Cliente: ClienteFactura{
				NombreRazonSocial: "CLIENTE TEST", CodigoTipoDocumentoIdentidad: 1, NumeroDocumento: "1234567", CodigoCliente: ptrStr("C-001"),
			}, Items: []ItemFactura{{ActividadEconomica: "473000", CodigoProductoSin: 12345, CodigoProducto: "P-001", Descripcion: "Item", Cantidad: 1, UnidadMedida: 1, PrecioUnitario: 100, SubTotal: 100}},
		}
		req.Items[0].DatosSector = datosDetalleDeEjemplo(profile)
		built, _, _, err := buildFacturaSDK(req, goSiat.EmisionOnline)
		if err != nil {
			t.Fatalf("sector %d modalidad %d: construir: %v", profile.Codigo, mode.code, err)
		}
		root, err := xmlRoot(built)
		if err != nil {
			t.Fatalf("sector %d modalidad %d: raíz XML: %v", profile.Codigo, mode.code, err)
		}
		if root != mode.root {
			t.Errorf("sector %d modalidad %d: raíz=%q, SDK=%q", profile.Codigo, mode.code, root, mode.root)
		}
	}
}

func xmlRoot(value any) (string, error) {
	data, err := xml.Marshal(value)
	if err != nil {
		return "", err
	}
	decoder := xml.NewDecoder(strings.NewReader(string(data)))
	for {
		token, err := decoder.Token()
		if err != nil {
			return "", err
		}
		if start, ok := token.(xml.StartElement); ok {
			return start.Name.Local, nil
		}
	}
}

func TestFacadeSelectorsAreExclusive(t *testing.T) {
	fixed := map[int]FachadaSDK{
		1: FachadaCompraVenta, 35: FachadaCompraVenta, 41: FachadaCompraVenta,
		22: FachadaTelecomunicaciones, 49: FachadaTelecomunicaciones,
		13: FachadaServicioBasico, 40: FachadaServicioBasico, 15: FachadaEntidadFinanciera,
		30: FachadaBoletoAereo, 24: FachadaDocumentoAjuste, 29: FachadaDocumentoAjuste,
		47: FachadaDocumentoAjuste, 48: FachadaDocumentoAjuste,
	}
	for _, profile := range PerfilesSector() {
		want, isFixed := fixed[profile.Codigo]
		if profile.Codigo == 24 && profile.Layout == string(LayoutNotaFiscalCreditoDebito) {
			isFixed = true
		}
		if isFixed {
			if profile.Facade.IsByModalidad() || profile.Facade.Fixed() != want {
				t.Errorf("sector %d layout %q: fachada=%s, fija=%v; esperada=%s", profile.Codigo, profile.Layout, profile.Facade, profile.Facade.IsByModalidad(), want)
			}
		} else if !profile.Facade.IsByModalidad() {
			t.Errorf("sector %d debe seleccionar fachada por modalidad, obtuvo %s", profile.Codigo, profile.Facade)
		}
	}
}

func (c sdkSectorCase) String() string {
	return fmt.Sprintf("%d/%s", c.code, c.name)
}
