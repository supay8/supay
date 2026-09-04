package main

import (
	"fmt"
	"log"
	"os"
	"strconv"

	appconfig "github.com/brandsrx/supay/internal/config"
	"github.com/brandsrx/supay/internal/domain"
	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/brandsrx/supay/internal/repository/postgres"
	"github.com/brandsrx/supay/internal/adapters/siat"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	fmt.Println("🔧 Seed SIAT: creando company y point of sale de prueba...")

	cfg := appconfig.Load()
	database.ConnectDB()

	companyRepo := postgres.NewPostgresCompanyRepository(database.DB)
	posRepo := postgres.NewPostgresPointOfSaleRepository(database.DB)

	// Crear u obtener company usando el NIT de la configuración SIAT
	nit := strconv.FormatInt(cfg.SIAT.Nit, 10)
	if nit == "" || nit == "0" {
		log.Fatalf("SIAT_NIT no está definido en el entorno; configura tu .env antes de correr este seed")
	}

	ambiente := domain.EnvironmentPiloto
	if cfg.SIAT.CodigoAmbiente == siat.AmbienteProduccion {
		ambiente = domain.EnvironmentProduccion
	}

	company, err := companyRepo.GetByNit(nit)
	if err != nil {
		company = &domain.Company{
			Nit:           nit,
			BusinessName:  "Supay Seed Company",
			CodigoSistema: cfg.SIAT.CodigoSistema,
			Ambiente:      ambiente,
		}
		if err := companyRepo.Create(company); err != nil {
			log.Fatalf("Error creando company: %v", err)
		}
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
