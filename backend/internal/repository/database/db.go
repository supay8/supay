package database

import (
	"fmt"
	"log"
	"os"

	"github.com/brandsrx/supay/internal/models"
	"github.com/joho/godotenv"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

func ConnectDB() {
	// Cargar variables del archivo .env si existe
	_ = godotenv.Load()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("Environment variable DATABASE_URL is not set. Please set it in your .env file or environment.")
	}

	var err error
	DB, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Muestra las queries SQL en la terminal para debug
	})
	if err != nil {
		log.Fatalf("Error of connection to PostgreSQL: %v", err)
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
		&models.PointOfSale{},
		&models.Cufd{},
		&models.ContingencyEvent{},
		&models.Customer{},
		&models.Invoice{},
		&models.InvoiceItem{},
		&models.InvoiceEvent{},
	)
	if err != nil {
		log.Fatalf("❌ Error al ejecutar migraciones: %v", err)
	}

	// Los índices únicos compuestos declarados con el patrón "_ struct{}" no los crea
	// AutoMigrate. Se garantizan aquí explícitamente (idempotente).
	// point_of_sales: unique (company_id, codigo_sucursal, codigo_punto_venta)
	if err := DB.Migrator().CreateIndex(&models.PointOfSale{}, "idx_company_sucursal_pv"); err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_company_sucursal_pv: %v", err)
	}

	// invoices: unique (point_of_sale_id, invoice_number).
	// Se elimina el índice previo (no único, solo sobre invoice_number) creado por el patrón viejo.
	DB.Exec("DROP INDEX IF EXISTS idx_pos_invoice_num")
	if err := DB.Migrator().CreateIndex(&models.Invoice{}, "idx_pos_invoice_num"); err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_pos_invoice_num: %v", err)
	}

	fmt.Println("✨ ¡Tablas migradas y listas en PostgreSQL!")
}
