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

// ConnectDB abre la conexión a PostgreSQL, configura el pool y devuelve la
// instancia de *gorm.DB. También mantiene la variable global DB para
// compatibilidad con código existente; el objetivo es que el contenedor de
// dependencias sea la única fuente de verdad en el futuro.
func ConnectDB() *gorm.DB {
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
		// PrepareStmt cachea prepared statements en pgx (reduce parse/plan
		// en queries repetidas como SELECT MAX invoice_number). Seguro sin
		// PgBouncer en modo transaction (no usado actualmente).
		PrepareStmt: true,
		// SkipDefaultTransaction evita BEGIN/COMMIT implícitos por cada
		// Create/Update; ya usamos tx explícitas donde se necesita
		// (invoice_repo: pg_advisory_xact_lock). Ahorra ~1 roundtrip.
		SkipDefaultTransaction: true,
	})
	if err != nil {
		log.Fatalf("Error of connection to PostgreSQL: %v", err)
	}

	// Tuning del pool database/sql para 50 VUs / 1-2 instancias.
	// max_connections en nube ~100; 60 deja margen para overhead y 2da instancia (60*2=120 -> ajustar max_connections a 150 si escalas).
	sqlDB, err := DB.DB()
	if err != nil {
		log.Fatalf("No se pudo obtener sql.DB del pool GORM: %v", err)
	}
	sqlDB.SetMaxOpenConns(60)                  // 50 VUs * 1.2 buffer
	sqlDB.SetMaxIdleConns(15)                  // 25% de MaxOpen, cubre burst sin churn
	sqlDB.SetConnMaxLifetime(30 * time.Minute) // recicla antes que LB/RDS cierre conns stale
	sqlDB.SetConnMaxIdleTime(5 * time.Minute)  // libera idles tras burst nocturno
	log.Printf("🔌 Pool DB configurado: MaxOpen=60 MaxIdle=15 MaxLifetime=30m MaxIdleTime=5m")

	// Configurar la zona horaria de la sesión PostgreSQL a America/La_Paz para
	// que las consultas y visualizaciones de timestamps muestren la hora de Bolivia.
	if err := DB.Exec("SET TIME ZONE 'America/La_Paz'").Error; err != nil {
		log.Printf("⚠️ No se pudo configurar Timezone La Paz en PostgreSQL: %v (se usa UTC por defecto)", err)
	}

	fmt.Println("Connection stablished with PostgreSQL database successfully!")

	return DB
}
func Migrate() error {
	log.Println("🔄 Ejecutando migraciones de base de datos...")

	// Tabla de control de backfills: debe existir ANTES del AutoMigrate
	// principal porque los backfills de clientes/facturas corridos abajo
	// preparan los datos para el esquema nuevo (el modelo declara
	// invoices.customer_id NOT NULL; si quedaran filas sin vincular,
	// AutoMigrate fallaría).
	if err := DB.AutoMigrate(&dataMigration{}); err != nil {
		log.Fatalf("❌ Error al crear la tabla data_migrations: %v", err)
		return err
	}

	// El índice único global idx_company_codigo_cliente (una sola columna,
	// sin company_id) impedía que dos empresas compartan un cliente con el
	// mismo documento. Debe caer antes del backfill, que crea clientes
	// derivados de receivers posiblemente de otras empresas.
	DB.Exec("DROP INDEX IF EXISTS idx_company_codigo_cliente")

	// Backfill: vincular las facturas históricas del modo receiver (ahora
	// eliminado) a clientes reales y completar codigo_cliente.
	runDataMigration("link_receiver_invoices_to_customers", backfillReceiverInvoices)
	runDataMigration("backfill_customers_codigo_cliente", backfillCodigoCliente)
	runDataMigration("dedup_customers_by_document", dedupCustomersByDocument)

	err := DB.AutoMigrate(
		&models.Company{},
		&models.ApiKey{},
		&models.Branch{},
		&models.TipoPuntoVenta{},
		&models.PointOfSale{},
		&models.Cufd{},
		&models.Catalog{},
		&models.SinProduct{},
		&models.SiatActividad{},
		&models.SiatLeyendaFactura{},
		&models.SiatActividadDocSector{},
		&models.CatalogSyncState{},
		&models.Product{},
		&models.ProductMapping{},
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
		return err
	}

	runDataMigration("fix_cufd_valid_to_plus_4h", func(tx *gorm.DB) error {
		// El SDK go-siat parsea fechaVigencia (hora de pared de Bolivia) como
		// UTC, por lo que los valid_to históricos quedaron 4 horas antes del
		// instante real. Se reajustan a la hora local de Bolivia (UTC-4).
		return tx.Exec("UPDATE cufds SET valid_to = valid_to + interval '4 hours'").Error
	})

	runDataMigration("migrate_productos_servicios_to_sin_products", func(tx *gorm.DB) error {
		if err := tx.Exec(`
            INSERT INTO sin_products (id, company_id, codigo_actividad, codigo_producto_sin, descripcion, active, synced_at, created_at, updated_at)
            SELECT DISTINCT ON (company_id, codigo)
                gen_random_uuid(), company_id, 0, codigo, descripcion, true, synced_at, created_at, now()
            FROM catalogs
            WHERE tipo = 'productosServicios'
            ORDER BY company_id, codigo, created_at DESC
            ON CONFLICT (company_id, codigo_actividad, codigo_producto_sin) DO UPDATE
            SET descripcion = EXCLUDED.descripcion, synced_at = EXCLUDED.synced_at, active = true, updated_at = now()
        `).Error; err != nil {
			return err
		}
		// Desde esta migración, productosServicios tiene una única fuente oficial.
		return tx.Exec("DELETE FROM catalogs WHERE tipo = 'productosServicios'").Error
	})

	// Los índices únicos compuestos declarados con el patrón "_ struct{}" no los crea
	// AutoMigrate. Se garantizan aquí explícitamente (idempotente).
	// point_of_sales: unique (company_id, codigo_sucursal, codigo_punto_venta)
	if err := DB.Migrator().CreateIndex(&models.PointOfSale{}, "idx_company_sucursal_pv"); err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_company_sucursal_pv: %v", err)
		return err

	}

	// branches: unique (company_id, codigo_sucursal)
	if err := DB.Migrator().CreateIndex(&models.Branch{}, "idx_company_sucursal"); err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_company_sucursal: %v", err)
		return err

	}
	// invoices: unique (point_of_sale_id, invoice_number).
	// Se elimina el índice previo (no único, solo sobre invoice_number) creado por el patrón viejo.
	DB.Exec("DROP INDEX IF EXISTS idx_pos_invoice_num")
	if err := DB.Migrator().CreateIndex(&models.Invoice{}, "idx_pos_invoice_num"); err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_pos_invoice_num: %v", err)
		return err
	}

	// invoices: unique (point_of_sale_id, idempotency_key). Protege contra
	// duplicados por retries del cliente con el mismo Idempotency-Key.
	if err := DB.Migrator().CreateIndex(&models.Invoice{}, "idx_invoice_idem_key"); err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_invoice_idem_key: %v", err)
		return err
	}

	// customers: idx_company_doc era NO único (permitía duplicados por
	// documento). Se reemplaza por el único (company_id, document_type,
	// document_number) declarado en el modelo; el dedup previo garantiza que
	// no hay duplicados.
	DB.Exec("DROP INDEX IF EXISTS idx_company_doc")
	DB.Exec("DROP INDEX IF EXISTS idx_company_fiscal_identity") // Por si acaso existiera de antes

	if err := DB.Exec(`
        CREATE UNIQUE INDEX idx_company_fiscal_identity 
        ON customers (
            company_id, 
            document_type, 
            document_number, 
            COALESCE(complement, ''), 
            name
        )
    `).Error; err != nil {
		log.Fatalf("❌ Error al crear el índice único idx_company_fiscal_identity: %v", err)
		return err
	}

	// invoices: eliminar el snapshot duplicado del receptor (violación de
	// 3FN frente a customers). AutoMigrate no elimina columnas; el backfill
	// previo ya vinculó toda factura a su cliente.
	for _, column := range []string{"receiver_name", "receiver_document_type", "receiver_document", "receiver_complement", "receiver_email"} {
		if err := DB.Exec("ALTER TABLE invoices DROP COLUMN IF EXISTS " + column).Error; err != nil {
			log.Fatalf("❌ Error al eliminar la columna invoices.%s: %v", column, err)
			return err
		}
	}

	// Aserción final del esquema: toda factura referencia un cliente.
	if err := DB.Exec("ALTER TABLE invoices ALTER COLUMN customer_id SET NOT NULL").Error; err != nil {
		log.Fatalf("❌ Error al aplicar NOT NULL a invoices.customer_id: %v (¿quedan facturas sin cliente?)", err)
		return err
	}

	// FK explícita invoices.customer_id → customers.id (idempotente).
	if err := DB.Exec(`DO $$ BEGIN
		IF NOT EXISTS (SELECT 1 FROM pg_constraint WHERE conname = 'fk_invoices_customer') THEN
			ALTER TABLE invoices ADD CONSTRAINT fk_invoices_customer
				FOREIGN KEY (customer_id) REFERENCES customers(id);
		END IF;
	END $$`).Error; err != nil {
		log.Fatalf("❌ Error al crear la FK fk_invoices_customer: %v", err)
		return err
	}

	// Regla de negocio reforzada en la BD: el cliente fiscal es inmutable
	// tras facturar (y DELETE bloqueado si tiene facturas).
	if err := ApplyCustomerImmutability(DB); err != nil {
		log.Fatalf("❌ Error al instalar la inmutabilidad de clientes: %v", err)
		return err
	}

	return nil
}

// ApplyCustomerImmutability instala el trigger que bloquea la modificación de
// los campos fiscales (name, document_type, document_number, complement) y el
// DELETE de clientes que tienen facturas asociadas, además de documentar la
// regla en el catálogo del sistema (COMMENT). Exportado para que los tests de
// integración lo instalen también en su BD de pruebas.
func ApplyCustomerImmutability(db *gorm.DB) error {
	if err := db.Exec(`
		CREATE OR REPLACE FUNCTION enforce_customer_immutability() RETURNS trigger AS $$
		BEGIN
			IF EXISTS (SELECT 1 FROM invoices WHERE customer_id = OLD.id) THEN
				IF TG_OP = 'DELETE' THEN
					RAISE EXCEPTION 'el cliente % tiene facturas asociadas; no se puede eliminar', OLD.id;
				END IF;
				IF OLD.company_id IS DISTINCT FROM NEW.company_id
					OR OLD.document_type IS DISTINCT FROM NEW.document_type
					OR OLD.document_number IS DISTINCT FROM NEW.document_number
					OR OLD.complement IS DISTINCT FROM NEW.complement
					OR OLD.name IS DISTINCT FROM NEW.name THEN
					RAISE EXCEPTION 'el cliente % tiene facturas asociadas; sus campos fiscales (name, document_type, document_number, complement) son inmutables: cree un nuevo cliente', OLD.id;
				END IF;
			END IF;
			RETURN COALESCE(NEW, OLD);
		END;
		$$ LANGUAGE plpgsql;
	`).Error; err != nil {
		return err
	}
	if err := db.Exec("DROP TRIGGER IF EXISTS trg_customers_immutability ON customers").Error; err != nil {
		return err
	}
	if err := db.Exec(`
		CREATE TRIGGER trg_customers_immutability
			BEFORE UPDATE OR DELETE ON customers
			FOR EACH ROW EXECUTE FUNCTION enforce_customer_immutability();
	`).Error; err != nil {
		return err
	}
	comments := []string{
		`COMMENT ON TABLE customers IS 'Registro fiscal del receptor (create-only). Tras tener facturas, name/document_type/document_number/complement son inmutables (trg_customers_immutability); si cambian, se crea un nuevo cliente. codigo_cliente se deriva: UPPER(document_type) || document_number.'`,
		`COMMENT ON COLUMN customers.codigo_cliente IS 'Derivado y fijado al crear: UPPER(document_type) || document_number. No se modifica después.'`,
		`COMMENT ON COLUMN invoices.customer_id IS 'Receptor de la factura: FK a customers, única fuente de verdad de los datos fiscales (sin snapshot duplicado).'`,
	}
	for _, comment := range comments {
		if err := db.Exec(comment).Error; err != nil {
			return err
		}
	}
	return nil
}

// backfillReceiverInvoices vincula las facturas históricas creadas por el modo
// receiver (customer_id NULL + columnas receiver_*) a clientes reales: busca
// el cliente por (empresa, documento) y lo crea a partir del snapshot si no
// existe. Falla rápido si al final quedan facturas sin cliente.
func backfillReceiverInvoices(tx *gorm.DB) error {
	// BD recién creada (sin tablas) o ya migrada (sin columnas receiver).
	if !tx.Migrator().HasTable("invoices") || !tx.Migrator().HasTable("customers") {
		return nil
	}
	var hasReceiver bool
	if err := tx.Raw(`SELECT EXISTS (
		SELECT 1 FROM information_schema.columns
		WHERE table_name = 'invoices' AND column_name = 'receiver_document')`).Scan(&hasReceiver).Error; err != nil {
		return err
	}
	if !hasReceiver {
		return nil
	}

	// 1) Crear los clientes faltantes a partir del snapshot receiver (se toma
	// el dato del borrador más antiguo de cada documento).
	if err := tx.Exec(`
		INSERT INTO customers (id, company_id, document_type, document_number, complement, name, codigo_cliente, created_at)
		SELECT DISTINCT ON (i.company_id, i.receiver_document_type, i.receiver_document)
			gen_random_uuid(),
			i.company_id,
			i.receiver_document_type,
			i.receiver_document,
			(SELECT i2.receiver_complement FROM invoices i2
				WHERE i2.company_id = i.company_id
					AND i2.receiver_document_type = i.receiver_document_type
					AND i2.receiver_document = i.receiver_document
					AND i2.customer_id IS NULL
				ORDER BY i2.created_at ASC LIMIT 1),
			(SELECT i2.receiver_name FROM invoices i2
				WHERE i2.company_id = i.company_id
					AND i2.receiver_document_type = i.receiver_document_type
					AND i2.receiver_document = i.receiver_document
					AND i2.customer_id IS NULL
				ORDER BY i2.created_at ASC LIMIT 1),
			UPPER(i.receiver_document_type) || i.receiver_document,
			now()
		FROM invoices i
		WHERE i.customer_id IS NULL
			AND i.receiver_document_type IS NOT NULL AND i.receiver_document_type <> ''
			AND i.receiver_document IS NOT NULL AND i.receiver_document <> ''
			AND i.company_id IS NOT NULL
			AND NOT EXISTS (
				SELECT 1 FROM customers c
				WHERE c.company_id = i.company_id
					AND c.document_type = i.receiver_document_type
					AND c.document_number = i.receiver_document)
		ON CONFLICT DO NOTHING
	`).Error; err != nil {
		return err
	}

	// 2) Vincular las facturas a su cliente.
	if err := tx.Exec(`
		UPDATE invoices i
		SET customer_id = c.id
		FROM customers c
		WHERE i.customer_id IS NULL
			AND c.company_id = i.company_id
			AND c.document_type = i.receiver_document_type
			AND c.document_number = i.receiver_document
	`).Error; err != nil {
		return err
	}

	// 3) Fail-fast: no deben quedar facturas sin cliente (datos corruptos sin
	// snapshot receiver); exigir corrección manual antes de continuar.
	var orphans int64
	if err := tx.Raw("SELECT COUNT(*) FROM invoices WHERE customer_id IS NULL").Scan(&orphans).Error; err != nil {
		return err
	}
	if orphans > 0 {
		return fmt.Errorf("quedan %d facturas con customer_id NULL y sin snapshot receiver para vincular; corríjalas manualmente (UPDATE invoices SET customer_id = <id>) antes de migrar", orphans)
	}
	return nil
}

// backfillCodigoCliente completa el codigo_cliente histórico (antes era
// opcional y se completaba perezosamente vía Update, vía que ya no existe).
func backfillCodigoCliente(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("customers") {
		return nil
	}
	return tx.Exec(`
		UPDATE customers
		SET codigo_cliente = UPPER(document_type) || document_number
		WHERE codigo_cliente IS NULL OR codigo_cliente = ''
	`).Error
}

// dedupCustomersByDocument elimina duplicados históricos por (empresa,
// documento) — el índice idx_company_doc no era único y podía admitirlos; el
// índice único nuevo los rechazaría. Keeper: el cliente con facturas (o el más
// antiguo); las facturas de los duplicados se re-apuntan al keeper.
func dedupCustomersByDocument(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("customers") || !tx.Migrator().HasTable("invoices") {
		return nil
	}
	if err := tx.Exec(`
		WITH ranked AS (
			SELECT c.id, c.company_id, c.document_type, c.document_number,
				EXISTS (SELECT 1 FROM invoices i WHERE i.customer_id = c.id) AS has_invoices,
				ROW_NUMBER() OVER (
					PARTITION BY c.company_id, c.document_type, c.document_number
					ORDER BY EXISTS (SELECT 1 FROM invoices i WHERE i.customer_id = c.id) DESC, c.created_at ASC
				) AS rn
			FROM customers c
		),
		pairs AS (
			SELECT d.id AS dupe_id, k.id AS keeper_id
			FROM ranked d
			JOIN ranked k
				ON k.company_id = d.company_id
					AND k.document_type = d.document_type
					AND k.document_number = d.document_number
					AND k.rn = 1 AND d.rn > 1
		)
		UPDATE invoices i
		SET customer_id = p.keeper_id
		FROM pairs p
		WHERE i.customer_id = p.dupe_id
	`).Error; err != nil {
		return err
	}
	return tx.Exec(`
		WITH ranked AS (
			SELECT c.id,
				EXISTS (SELECT 1 FROM invoices i WHERE i.customer_id = c.id) AS has_invoices,
				ROW_NUMBER() OVER (
					PARTITION BY c.company_id, c.document_type, c.document_number
					ORDER BY EXISTS (SELECT 1 FROM invoices i WHERE i.customer_id = c.id) DESC, c.created_at ASC
				) AS rn
			FROM customers c
		)
		DELETE FROM customers
		WHERE id IN (SELECT id FROM ranked WHERE rn > 1)
	`).Error
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
