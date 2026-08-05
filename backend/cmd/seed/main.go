package main

import (
	"fmt"
	"log"
	"os"

	appconfig "github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/brandsrx/supay/internal/repository/postgres"
)

func main() {
	fmt.Println("🔧 Seed SIAT: creando company y point of sale de prueba...")

	cfg := appconfig.Load()
	database.ConnectDB()

	companyRepo := postgres.NewPostgresCompanyRepository(database.DB)
	posRepo := postgres.NewPostgresPointOfSaleRepository(database.DB)

	// Crear o obtener company usando NIT de env
	nit := cfg.SIAT.CloneHeaders()["nit"]
	if nit == "" {
		// Fallback: usar valor de SIAT_NIT del entorno directamente
		nit = getenv("SIAT_NIT", "")
	}

	if nit == "" {
		log.Fatalf("SIAT_NIT no está definido en el entorno; configura tu .env antes de correr este seed")
	}

	company := &domain.Company{
		Nit:           nit,
		BusinessName:  "Supay Seed Company",
		CodigoSistema: getenv("SIAT_CODIGO_SISTEMA", ""),
		Ambiente:      domain.EnvironmentPiloto,
	}

	if err := companyRepo.Create(company); err != nil {
		log.Fatalf("Error creando company: %v", err)
	}

	pos := &domain.PointOfSale{
		CompanyId:      company.ID,
		CodigoSucursal: parseInt(getenv("SIAT_CODIGO_SUCURSAL", "0"), 0),
		Description:    "Punto de venta seed",
		IsActive:       true,
	}

	if err := posRepo.Create(pos); err != nil {
		log.Fatalf("Error creando punto de venta: %v", err)
	}

	fmt.Printf("✅ Seed completado. Company ID=%s NIT=%s\n", company.ID, company.Nit)
	fmt.Printf("✅ Punto de venta creado. ID=%s CodigoSucursal=%d CodigoPuntoVenta=%d\n", pos.ID, pos.CodigoSucursal, pos.CodigoPuntoVenta)
	fmt.Println("Ahora puedes ejecutar el servidor y llamar a POST /siat/cuis/{companyId}/{pointOfSaleId}")
}

// helpers
func getenv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok {
		trimmed := val
		if trimmed != "" {
			return trimmed
		}
	}
	return fallback
}

func parseInt(s string, fallback int) int {
	var v int
	_, err := fmt.Sscanf(s, "%d", &v)
	if err != nil {
		return fallback
	}
	return v
}
