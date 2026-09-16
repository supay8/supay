package main

import (
	"log"

	"github.com/brandsrx/supay/internal/repository/database"
	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()
	db := database.ConnectDB()

	if err := database.MigrateDB(db); err != nil {
		log.Fatalf("migración fallida: %v", err)
	}

	type migrationState struct {
		Version int
		Dirty   bool
	}
	var state migrationState
	if err := db.Raw("SELECT version, dirty FROM schema_migrations LIMIT 1").Scan(&state).Error; err != nil {
		log.Fatalf("migración aplicada, pero no se pudo verificar schema_migrations: %v", err)
	}

	var columns []string
	if err := db.Raw(`
		SELECT column_name
		FROM information_schema.columns
		WHERE table_schema = 'public' AND table_name = 'tenant_configs'
		ORDER BY ordinal_position
	`).Scan(&columns).Error; err != nil {
		log.Fatalf("migración aplicada, pero no se pudo verificar tenant_configs: %v", err)
	}

	log.Printf("migración completada: version=%d dirty=%t tenant_configs=%v", state.Version, state.Dirty, columns)
}
