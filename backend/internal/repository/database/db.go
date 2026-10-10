package database

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
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

type poolConfig struct {
	maxOpen int
	maxIdle int
}

// migrationFiles forma parte del binario para que el servidor pueda ejecutar
// migraciones sin depender del directorio de trabajo ni de archivos externos.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

// ConnectDB keeps the legacy CLI connection for migrate/seed commands.
func ConnectDB() (*gorm.DB, error) {
	_ = godotenv.Load()
	db, err := Open(os.Getenv("DATABASE_URL"))
	if err != nil {
		return nil, err
	}
	DB = db
	return db, nil
}

// Open constructs an owned pool and propagates errors to its caller.
func Open(dsn string) (*gorm.DB, error) {
	return OpenForMode(dsn, os.Getenv("DEPLOYMENT_MODE"))
}

// OpenForMode uses the factory's deployment mode for pool limits.
func OpenForMode(dsn, deploymentMode string) (*gorm.DB, error) {
	if strings.TrimSpace(dsn) == "" {
		return nil, errors.New("DATABASE_URL es obligatorio")
	}
	pool, err := poolConfigForMode(deploymentMode)
	if err != nil {
		return nil, fmt.Errorf("configuración del pool: %w", err)
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
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
		if db != nil {
			if pool, poolErr := db.DB(); poolErr == nil {
				_ = pool.Close()
			}
		}
		return nil, fmt.Errorf("conexión PostgreSQL: %w", err)
	}

	sqldb, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("pool GORM: %w", err)
	}
	sqldb.SetMaxOpenConns(pool.maxOpen)
	sqldb.SetMaxIdleConns(pool.maxIdle)
	sqldb.SetConnMaxLifetime(30 * time.Minute) // recicla antes que LB/RDS cierre conns stale
	sqldb.SetConnMaxIdleTime(5 * time.Minute)  // libera idles tras burst nocturno
	log.Printf("🔌 Pool db configurado: MaxOpen=%d MaxIdle=%d MaxLifetime=30m MaxIdleTime=5m", pool.maxOpen, pool.maxIdle)

	// Configurar la zona horaria de la sesión PostgreSQL a America/La_Paz para
	// que las consultas y visualizaciones de timestamps muestren la hora de Bolivia.
	if err := db.Exec("SET TIME ZONE 'America/La_Paz'").Error; err != nil {
		log.Printf("⚠️ No se pudo configurar Timezone La Paz en PostgreSQL: %v (se usa UTC por defecto)", err)
	}

	fmt.Println("Connection stablished with PostgreSQL database successfully!")

	return db, nil
}

func poolConfigFromEnv() (poolConfig, error) { return poolConfigForMode(os.Getenv("DEPLOYMENT_MODE")) }

func poolConfigForMode(mode string) (poolConfig, error) {
	cloud := strings.EqualFold(strings.TrimSpace(mode), "cloud")
	defaults := poolConfig{maxOpen: 60, maxIdle: 15}
	if cloud {
		defaults = poolConfig{maxOpen: 10, maxIdle: 5}
	}
	maxOpen, err := positiveEnvInt("DB_MAX_OPEN", defaults.maxOpen)
	if err != nil {
		return poolConfig{}, err
	}
	maxIdle, err := positiveEnvInt("DB_MAX_IDLE", defaults.maxIdle)
	if err != nil {
		return poolConfig{}, err
	}
	if cloud && maxOpen > 15 {
		return poolConfig{}, fmt.Errorf("DB_MAX_OPEN=%d excede el máximo 15 para Cloud Run", maxOpen)
	}
	if maxIdle > maxOpen {
		return poolConfig{}, fmt.Errorf("DB_MAX_IDLE=%d no puede superar DB_MAX_OPEN=%d", maxIdle, maxOpen)
	}
	return poolConfig{maxOpen: maxOpen, maxIdle: maxIdle}, nil
}

func positiveEnvInt(key string, fallback int) (int, error) {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s debe ser un entero mayor a cero", key)
	}
	return value, nil
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
