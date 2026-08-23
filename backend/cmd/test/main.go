package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/ron86i/go-siat/v2"
	"github.com/ron86i/go-siat/v2/pkg/models"
)

func formatUtc(t string) time.Time {
	// Si tu string no tiene zona horaria, usa este layout
	layout := "2006-01-02T15:04:05.000"
	tm, err := time.Parse(layout, t)
	if err != nil {
		log.Fatalf("Error parseando fecha: %v", err)
	}
	// Asegurarse de devolverla en UTC
	return tm.UTC()
}
func main() {
	fmt.Println("=== Extracción Masiva de Catálogos SIAT a JSON ===")

	nit := int64(9971522011)
	token := "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzUxMiJ9.eyJzdWIiOiJyYW1pdHByNTNAZ21haWwuY29tIiwiY29kaWdvU2lzdGVtYSI6IjIyODQ1MkMzOEVEODczOTQwOEFCNiIsIm5pdCI6Ikg0c0lBQUFBQUFBQUFMTzBORGMwTlRJeU1EUUVBRVhzZmhJS0FBQUEiLCJpZCI6NjE1NjMyNSwiZXhwIjoxNzg4MTkyNDEwLCJpYXQiOjE3ODY2NTE1ODAsIm5pdERlbGVnYWRvIjo5OTcxNTIyMDExLCJzdWJzaXN0ZW1hIjoiU0ZFIn0.PuB5t3vfE3iBf_mT5MvBt2tDyTALnVRBtCX1Cf0KYAgaWNQeT7L3hrrlYY2Ggdsn8cVPnQJxemJfAvRs86Svvw"
	codigoSistema := "228452C38ED8739408AB6"
	baseURL := "https://pilotosiatservicios.impuestos.gob.bo/v2"
	cuis := "4FF1AED1"

	cfg := siat.Config{
		Nit:            nit,
		Token:          token,
		CodigoSistema:  codigoSistema,
		CodigoAmbiente: siat.AmbientePruebas, // 2 = Piloto
		BaseURL:        baseURL,
	}

	s, err := siat.New(cfg)
	if err != nil {
		log.Fatalf("Error inicializando cliente: %v", err)
	}

	reqCufd := models.NewCufdBuilder().WithCuis(cuis).WithCodigoModalidad(1).WithCodigoSucursal(0).WithCodigoPuntoVenta(1).Build()
	// 1. Solicitamos el CUFD fresco
	cufdResp, _ := s.Codigos().SolicitudCufd(context.Background(), reqCufd)
	cufd := cufdResp.Body.Content.RespuestaCufd.Codigo
	fmt.Println("Cufd obtenido:", cufd)

	// 2. Esperamos 3 segundos para asegurar que el tiempo del servidor del SIAT
	// reconozca que el CUFD ya es válido y está en el pasado.
	fmt.Println("Sincronizando tiempos con el SIAT...")
	time.Sleep(3 * time.Second)

	// 3. Marcamos el INICIO del evento
	fechaStarted := time.Now().UTC()
	fmt.Println("Inicio del evento:", fechaStarted.Format("2006-01-02T15:04:05.000"))

	// 4. Simulamos que el corte dura 5 segundos
	fmt.Println("Simulando contingencia...")
	time.Sleep(5 * time.Second)

	// 5. Marcamos el FIN del evento
	fechaEnd := time.Now().UTC()
	fmt.Println("Fin del evento:", fechaEnd.Format("2006-01-02T15:04:05.000"))

	// 6. Esperamos 2 segunditos extra antes de enviar para que el SIAT
	// no rechace el evento por ser una "fecha en el futuro" respecto a su reloj.
	time.Sleep(2 * time.Second)

	req := models.NewRegistroEventoSignificativoBuilder().
		WithCodigoSucursal(0).
		WithCodigoPuntoVenta(1).
		WithCuis(cuis).
		WithCufd(cufd). // Usamos el CUFD fresco en la cabecera
		WithCodigoMotivoEvento(1).
		WithDescripcion("CORTE DEL SERVICIO DE INTERNET").
		WithFechaInicio(fechaStarted).
		WithFechaFin(fechaEnd).
		WithCufdEvento(cufd). // Reutilizamos el mismo CUFD fresco para el evento
		Build()

	x, err := s.Operaciones().RegistroEventosSignificativos(context.Background(), req)
	if err != nil {
		log.Fatalf("Error registrando evento significativo: %v", err)
	}

	data, _ := json.MarshalIndent(x, "", "  ")
	fmt.Println(string(data))
}
