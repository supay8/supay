package siat

import (
	"encoding/xml"
	"fmt"

	"github.com/brandsrx/supay/internal/models"
)

// InvoiceXMLParams agrupa los datos necesarios para construir el XML de una
// factura de compra-venta conforme a la estructura del SIAT
// (facturaElectronicaCompraVenta).
type InvoiceXMLParams struct {
	Invoice *models.Invoice

	Cuf                string
	Cufd               string
	CodigoControl      string
	DireccionSucursal  string
	Municipio          string
	Telefono           string
	PiePagina          string
	CodigoActividad    string // actividad económica del emisor (6 dígitos)
	Modalidad          int    // 1 = Electrónica en Línea
	TipoEmision        int    // 1 = Online
	CodigoDocumentoSector int
	CodigoPuntoVenta   int // código oficial del punto de venta ante el SIAT
	MontoTotalSujetoIva *float64 // nil => montoTotal
}

// BuildFacturaXML serializa la factura en XML SIAT (cabecera + detalle).
func BuildFacturaXML(p InvoiceXMLParams) ([]byte, error) {
	if p.Invoice == nil {
		return nil, fmt.Errorf("siat invoice xml: invoice es obligatoria")
	}
	if p.Cuf == "" {
		return nil, fmt.Errorf("siat invoice xml: cuf es obligatorio")
	}

	inv := p.Invoice
	company := inv.Company
	customer := inv.Customer

	codigoActividad := p.CodigoActividad
	documentSector := p.CodigoDocumentoSector
	if documentSector <= 0 {
		documentSector = 1 // Factura de Compra Venta
	}
	modalidad := p.Modalidad
	if modalidad <= 0 {
		modalidad = 1
	}
	tipoEmision := p.TipoEmision
	if tipoEmision <= 0 {
		tipoEmision = 1
	}
	puntoVenta := p.CodigoPuntoVenta

	tipoDocIdentidad := DocumentTypeCode(customer.DocumentType)

	xmlInv := facturaElectronicaCompraVenta{
		XMLName: xml.Name{
			Local: "facturaElectronicaCompraVenta",
		},
		XmlnsXsi:       "http://www.w3.org/2001/XMLSchema-instance",
		SchemaLocation: "facturaElectronicaCompraVenta.xsd",
		Cabecera: cabecera{
			NitEmisor:                  company.Nit,
			RazonSocialEmisor:          company.BusinessName,
			Municipio:                  p.Municipio,
			Telefono:                   p.Telefono,
			NumeroFactura:              inv.InvoiceNumber,
			Cuf:                        p.Cuf,
			Cufd:                       p.Cufd,
			CodigoSucursal:             inv.PointOfSale.CodigoSucursal,
			Direccion:                  p.DireccionSucursal,
			CodigoPuntoVenta:           puntoVenta,
			FechaEmision:               inv.IssueDate.In(boliviaZone).Format("2006-01-02T15:04:05.000-07:00"),
			NombreRazonSocial:          customer.Name,
			CodigoTipoDocumentoIdentidad: tipoDocIdentidad,
			NumeroDocumento:            customer.DocumentNumber,
			Complemento:                nonNilStrPtr(customer.Complement),
			CodigoCliente:              customer.ID,
			CodigoMetodoPago:           inv.CodigoMetodoPago,
			NumeroTarjeta:              0,
			MontoTotal:                 inv.Total,
			MontoTotalSujetoIva:        montoSujetoIva(inv.Total, p.MontoTotalSujetoIva),
			CodigoMoneda:               inv.CodigoMoneda,
			TipoCambio:                 inv.TipoCambio,
			MontoTotalMoneda:           inv.Total,
			MontoGiftCard:              0,
			DescuentoAdicional:         inv.Discount,
			CodigoExcepcion:            0,
			Cafc:                       cafcNil{Nil: "true"},
			Leyenda:                    p.PiePagina,
			Usuario:                    company.Nit,
			CodigoDocumentoSector:      documentSector,
		},
	}

	for _, it := range inv.Items {
		actividad := codigoActividad
		if it.CodigoActividad != nil && *it.CodigoActividad != "" {
			actividad = *it.CodigoActividad
		}
		unidad := 1
		if it.UnitCode != nil && *it.UnitCode > 0 {
			unidad = *it.UnitCode
		}
		xmlInv.Detalle = append(xmlInv.Detalle, detalle{
			ActividadEconomica: actividad,
			CodigoProductoSin:  strPtr(it.CodigoProductoSin),
			CodigoProducto:     it.Code,
			Descripcion:        it.Description,
			Cantidad:           it.Quantity,
			UnidadMedida:       unidad,
			PrecioUnitario:     it.UnitPrice,
			MontoDescuento:     it.Discount,
			SubTotal:           it.Subtotal,
			NumeroSerie:        "0",
			NumeroImei:         "0",
		})
	}

	payload, err := xml.MarshalIndent(xmlInv, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("siat invoice xml: marshal: %w", err)
	}
	return payload, nil
}

// DocumentTypeCode mapea el tipo de documento del modelo a los códigos del
// catálogo SIAT "Tipos de documento de identidad".
func DocumentTypeCode(dt models.DocumentType) int {
	switch dt {
	case models.DocCI:
		return 1
	case models.DocCEX:
		return 2
	case models.DocPAS:
		return 3
	case models.DocOD:
		return 4
	case models.DocNIT:
		return 5
	default:
		return 1
	}
}

func montoSujetoIva(total float64, override *float64) float64 {
	if override != nil {
		return *override
	}
	return total
}

func strPtr(s *string) *string { return s }

// nonNilStrPtr devuelve un puntero no nulo al string, usando una referencia a
// string vacío cuando s es nil. El XSD del SIAT exige el elemento complemento
// (puede ir vacío) y Go omite el elemento cuando el puntero es nil.
func nonNilStrPtr(s *string) *string {
	if s == nil {
		e := ""
		return &e
	}
	return s
}

// --- Estructuras XML (namespaces SIAT) ---

type facturaElectronicaCompraVenta struct {
	XMLName  xml.Name
	XmlnsXsi string  `xml:"xmlns:xsi,attr"`
	SchemaLocation string `xml:"xsi:noNamespaceSchemaLocation,attr"`
	Cabecera cabecera  `xml:"cabecera"`
	Detalle  []detalle `xml:"detalle"`
}

// cafcNil serializa el elemento cafc como nulo (xsi:nil="true"): el XSD exige
// el elemento presente pero la emisión en línea no puede llevar valor CAFC.
type cafcNil struct {
	Nil  string `xml:"http://www.w3.org/2001/XMLSchema-instance nil,attr"`
	Text string `xml:",chardata"`
}

type cabecera struct {
	NitEmisor                    string   `xml:"nitEmisor"`
	RazonSocialEmisor            string   `xml:"razonSocialEmisor"`
	Municipio                    string   `xml:"municipio"`
	Telefono                     string   `xml:"telefono"`
	NumeroFactura                int      `xml:"numeroFactura"`
	Cuf                          string   `xml:"cuf"`
	Cufd                         string   `xml:"cufd"`
	CodigoSucursal               int      `xml:"codigoSucursal"`
	Direccion                    string   `xml:"direccion"`
	CodigoPuntoVenta             int      `xml:"codigoPuntoVenta"`
	FechaEmision                 string   `xml:"fechaEmision"`
	NombreRazonSocial            string   `xml:"nombreRazonSocial"`
	CodigoTipoDocumentoIdentidad int      `xml:"codigoTipoDocumentoIdentidad"`
	NumeroDocumento              string   `xml:"numeroDocumento"`
	Complemento                  *string  `xml:"complemento"`
	CodigoCliente               string   `xml:"codigoCliente"`
	CodigoMetodoPago             int      `xml:"codigoMetodoPago"`
	NumeroTarjeta                int64    `xml:"numeroTarjeta"`
	MontoTotal                   float64  `xml:"montoTotal"`
	MontoTotalSujetoIva          float64  `xml:"montoTotalSujetoIva"`
	CodigoMoneda                 int      `xml:"codigoMoneda"`
	TipoCambio                   float64  `xml:"tipoCambio"`
	MontoTotalMoneda             float64  `xml:"montoTotalMoneda"`
	MontoGiftCard                float64  `xml:"montoGiftCard"`
	DescuentoAdicional           float64  `xml:"descuentoAdicional"`
	CodigoExcepcion              int64    `xml:"codigoExcepcion"`
	Cafc                         cafcNil  `xml:"cafc"`
	Leyenda                      string   `xml:"leyenda"`
	Usuario                      string   `xml:"usuario"`
	CodigoDocumentoSector        int      `xml:"codigoDocumentoSector"`
}

type detalle struct {
	ActividadEconomica string   `xml:"actividadEconomica"`
	CodigoProductoSin  *string  `xml:"codigoProductoSin,omitempty"`
	CodigoProducto     string   `xml:"codigoProducto"`
	Descripcion        string   `xml:"descripcion"`
	Cantidad           float64  `xml:"cantidad"`
	UnidadMedida       int      `xml:"unidadMedida"`
	PrecioUnitario     float64  `xml:"precioUnitario"`
	MontoDescuento     float64  `xml:"montoDescuento"`
	SubTotal           float64  `xml:"subTotal"`
	NumeroSerie        string   `xml:"numeroSerie"`
	NumeroImei         string   `xml:"numeroImei"`
}
