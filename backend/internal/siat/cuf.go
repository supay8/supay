package siat

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

// CUFParams agrupa los datos necesarios para generar el Código Único de Factura
// conforme al algoritmo oficial del SIN/SIAT (Anexo Técnico "Generación CUF").
type CUFParams struct {
	Nit                  int64
	FechaHora            time.Time
	CodigoSucursal       int
	Modalidad            int // 1 = Electrónica en Línea, 2 = Computarizada en Línea
	TipoEmision          int // 1 = Online, 2 = Offline
	TipoFactura          int // 1 = con derecho a crédito fiscal
	CodigoDocumentoSector int
	NumeroFactura        int
	CodigoPuntoVenta     int
	CodigoControl        string // codigoControl devuelto por solicitudCufd
}

// GenerarCUF calcula el CUF oficial:
//
//  1. Se completan los campos con la longitud indicada (ceros a la izquierda)
//     y se concatenan: NIT(13) + FECHA(17) + SUCURSAL(4) + MODALIDAD(1) +
//     TIPO EMISIÓN(1) + TIPO FACTURA(1) + DOC SECTOR(2) + Nº FACTURA(10) +
//     PUNTO DE VENTA(4) = 53 dígitos.
//  2. Se adjunta el dígito autoverificador (Módulo 11).
//  3. La cadena se codifica a Base 16 (hexadecimal, mayúsculas).
//  4. Se concatena el código de control del CUFD.
func (p CUFParams) GenerarCUF() (string, error) {
	if p.Nit <= 0 {
		return "", fmt.Errorf("siat cuf: nit es obligatorio")
	}
	if p.FechaHora.IsZero() {
		return "", fmt.Errorf("siat cuf: fechaHora es obligatoria")
	}
	if p.CodigoControl == "" {
		return "", fmt.Errorf("siat cuf: codigoControl (CUFD) es obligatorio")
	}

	nitStr := fmt.Sprintf("%013d", p.Nit)
	fechaStr := p.FechaHora.Format("20060102150405.000")
	fechaStr = strings.ReplaceAll(fechaStr, ".", "")
	sucursalStr := fmt.Sprintf("%04d", p.CodigoSucursal)
	modalidadStr := fmt.Sprintf("%01d", p.Modalidad)
	tipoEmisionStr := fmt.Sprintf("%01d", p.TipoEmision)
	tipoFacturaStr := fmt.Sprintf("%01d", p.TipoFactura)
	docSectorStr := fmt.Sprintf("%02d", p.CodigoDocumentoSector)
	numeroFacturaStr := fmt.Sprintf("%010d", p.NumeroFactura)
	puntoVentaStr := fmt.Sprintf("%04d", p.CodigoPuntoVenta)

	cadena := nitStr + fechaStr + sucursalStr + modalidadStr + tipoEmisionStr +
		tipoFacturaStr + docSectorStr + numeroFacturaStr + puntoVentaStr

	digito := calculaDigitoMod11(cadena)
	cadena += digito

	bigInt := new(big.Int)
	if _, ok := bigInt.SetString(cadena, 10); !ok {
		return "", fmt.Errorf("siat cuf: cadena inválida para conversión base16")
	}
	cadenaHex := strings.ToUpper(bigInt.Text(16))

	return cadenaHex + p.CodigoControl, nil
}

// calculaDigitoMod11 implementa el dígito autoverificador Módulo 11 del SIAT:
// multiplicadores 2..9 en orden descendente desde el último dígito; si el
// residuo es 10 se usa "1", si es 11 se usa "0".
func calculaDigitoMod11(cadena string) string {
	suma := 0
	mult := 2
	for i := len(cadena) - 1; i >= 0; i-- {
		suma += mult * int(cadena[i]-'0')
		mult++
		if mult > 9 {
			mult = 2
		}
	}
	dig := suma % 11
	switch dig {
	case 10:
		return "1"
	case 11:
		return "0"
	default:
		return fmt.Sprintf("%d", dig)
	}
}
