package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/golang-migrate/migrate/v4"
	migratedatabase "github.com/golang-migrate/migrate/v4/database"
	migratepostgres "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/joho/godotenv"
	"github.com/riverqueue/river/riverdriver/riverdatabasesql"
	"github.com/riverqueue/river/rivermigrate"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var DB *gorm.DB

// migrationFiles forma parte del binario para que el servidor pueda ejecutar
// migraciones sin depender del directorio de trabajo ni de archivos externos.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

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
	if DB == nil {
		return errors.New("database is not connected")
	}
	return MigrateDB(DB)
}

// MigrateDB aplica todas las migraciones pendientes sobre una conexión GORM.
// Recibir la conexión explícitamente permite usar exactamente el mismo camino
// en tests de integración y en producción.
func MigrateDB(db *gorm.DB) error {
	if db == nil {
		return errors.New("database is not connected")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("obtener conexión SQL para migraciones: %w", err)
	}

	driver, err := migratepostgres.WithInstance(sqlDB, &migratepostgres.Config{})
	if err != nil {
		return fmt.Errorf("crear driver PostgreSQL de migraciones: %w", err)
	}
	source, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("abrir migraciones embebidas: %w", err)
	}
	m, err := migrate.NewWithInstance("iofs", source, "postgres", driver)
	if err != nil {
		return fmt.Errorf("inicializar golang-migrate: %w", err)
	}
	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		var migrationErr migratedatabase.Error
		var pgErr *pgconn.PgError
		if errors.As(err, &migrationErr) {
			pgErr = nil
			errors.As(migrationErr.OrigErr, &pgErr)
		}
		if pgErr != nil {
			return fmt.Errorf("aplicar migraciones: PostgreSQL %s (%s): %s; detalle: %s; sugerencia: %s", pgErr.Code, pgErr.Severity, pgErr.Message, pgErr.Detail, pgErr.Hint)
		}
		return fmt.Errorf("aplicar migraciones: %w", err)
	}

	// River owns and versions its queue schema. Running its bundled migrations
	// here keeps AUTO_MIGRATE and integration tests on one deterministic path.
	riverMigrator, err := rivermigrate.New(riverdatabasesql.New(sqlDB), nil)
	if err != nil {
		return fmt.Errorf("inicializar migraciones River: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if _, err := riverMigrator.Migrate(ctx, rivermigrate.DirectionUp, nil); err != nil {
		return fmt.Errorf("aplicar migraciones River: %w", err)
	}
	return nil
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
