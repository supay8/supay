package database

import (
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/brandsrx/supay/internal/models"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// dataMigration registra las migraciones de datos de una sola corrida para que
// sean idempotentes (no hay framework de migraciones para backfills).
type dataMigration struct {
	Name  string    `gorm:"column:name;type:varchar(200);primaryKey"`
	RunAt time.Time `gorm:"not null"`
}

func ConnectDB() {
	// Cargar variables del archivo .env si existe
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("Environment variable DATABASE_URL is not set. Please set it in your .env file or environment.")
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		// Nivel de log configurable vía LOG_LEVEL (debug|info|warn|error).
		// Por defecto Warn para no volcar queries con datos fiscales en producción.
		Logger: logger.Default.LogMode(gormLogLevel()),
	})
	if err != nil {
		log.Fatalf("Error of connection to PostgreSQL: %v", err)
	}

	// Configurar la zona horaria de la sesión PostgreSQL a America/La_Paz para
	// que las consultas y visualizaciones de timestamps muestren la hora de Bolivia.
	if err := DB.Exec("SET TIME ZONE 'America/La_Paz'").Error; err != nil {
		log.Printf("⚠️ No se pudo configurar Timezone La Paz en PostgreSQL: %v (se usa UTC por defecto)", err)
	}

	fmt.Println("Connection stablished with PostgreSQL database successfully!")
	// log.Println("🗑️ Borrando tablas viejas por conflicto de tipos...")
	// DB.Migrator().DropTable(
	// 	&models.InvoiceEvent{},
	// 	&models.InvoiceItem{},
	// 	&models.Invoice{},
	// 	&models.Customer{},
	// 	&models.ContingencyEvent{},
	// 	&models.Cufd{},
	// 	&models.PointOfSale{},
	// 	&models.Company{},
	// )
	// Migraciones automáticas: GORM creará/actualizará las tablas automáticamente
	log.Println("🔄 Ejecutando migraciones de base de datos...")
	err = DB.AutoMigrate(
		&models.Company{},
		&models.Branch{},
		&models.TipoPuntoVenta{},
		&models.PointOfSale{},
		&models.Cufd{},
		&models.Cuis{},
		&models.Catalog{},
		&models.ContingencyEvent{},
		&models.Customer{},
		&models.Invoice{},
		&models.InvoiceItem{},
		&models.InvoiceEvent{},
		&models.SentPackage{},
		&models.Certificate{},
		&dataMigration{},
	)
	if err != nil {
		log.Fatalf("❌ Error al ejecutar migraciones: %v", err)
	}

	runDataMigration("fix_cufd_valid_to_plus_4h", func(tx *gorm.DB) error {
		// El SDK go-siat parsea fechaVigencia (hora de pared de Bolivia) como
		// UTC, por lo que los valid_to históricos quedaron 4 horas antes del
		// instante real. Se reajustan a la hora local de Bolivia (UTC-4).
		return tx.Exec("UPDATE cufds SET valid_to = valid_to + interval '4 hours'").Error
	})

	// Los índices únicos compuestos declarados con el patrón "_ struct{}" no los crea
	// AutoMigrate. Se garantizan aquí explícitamente (idempotente).
	// point_of_sales: unique (company_id, codigo_sucursal, codigo_punto_venta)
	if err := DB.Migrator().CreateIndex(&models.PointOfSale{}, "idx_company_sucursal_pv"); err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_company_sucursal_pv: %v", err)
	}

	// branches: unique (company_id, codigo_sucursal)
	if err := DB.Migrator().CreateIndex(&models.Branch{}, "idx_company_sucursal"); err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_company_sucursal: %v", err)
	}

	// tipo_punto_ventas: unique (company_id, codigo_clasificador)
	if err := DB.Migrator().CreateIndex(&models.TipoPuntoVenta{}, "idx_company_tipo_pv"); err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_company_tipo_pv: %v", err)
	}

	// invoices: unique (point_of_sale_id, invoice_number).
	// Se elimina el índice previo (no único, solo sobre invoice_number) creado por el patrón viejo.
	DB.Exec("DROP INDEX IF EXISTS idx_pos_invoice_num")
	if err := DB.Migrator().CreateIndex(&models.Invoice{}, "idx_pos_invoice_num"); err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_pos_invoice_num: %v", err)
	}

	fmt.Println("✨ ¡Tablas migradas y listas en PostgreSQL!")
}

// gormLogLevel traduce LOG_LEVEL al nivel de logger de GORM.
func gormLogLevel() logger.LogLevel {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		return logger.Info
	case "error":
		return logger.Error
	default:
		return logger.Warn
	}
}

// runDataMigration ejecuta un backfill de datos una sola vez, registrándolo en
// data_migrations para que correrla de nuevo no repita el cambio.
func runDataMigration(name string, fn func(tx *gorm.DB) error) {
	tx := DB.Begin()
	defer tx.Rollback()

	res := tx.Exec("INSERT INTO data_migrations(name, run_at) SELECT ?, now() WHERE NOT EXISTS (SELECT 1 FROM data_migrations WHERE name = ?)", name, name)
	if res.Error != nil {
		log.Fatalf("❌ Error al registrar migración de datos %q: %v", name, res.Error)
	}
	if res.RowsAffected == 0 {
		return
	}
	if err := fn(tx); err != nil {
		log.Fatalf("❌ Error al ejecutar migración de datos %q: %v", name, err)
	}
	if err := tx.Commit().Error; err != nil {
		log.Fatalf("❌ Error al confirmar migración de datos %q: %v", name, err)
	}
	log.Printf("✅ Migración de datos %q ejecutada", name)
}
