// Command migrate applies the gwx SQL migrations through the ledger-aware
// store.Migrate runner. It is idempotent: already-applied files are skipped, so
// it is safe to run on every deploy and on an already-populated database.
package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"

	"gwx/internal/store"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("[MIGRATE] no .env loaded, using process environment")
	}

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		log.Fatal("[FATAL] DATABASE_URL is required")
	}
	dir := "migrations"
	if len(os.Args) > 1 {
		dir = os.Args[1]
	}

	ctx := context.Background()
	db, err := store.NewPostgres(ctx, dsn)
	if err != nil {
		log.Fatalf("[FATAL] postgres: %v", err)
	}
	defer db.Close()

	report, err := store.Migrate(ctx, db, dir)
	if err != nil {
		log.Fatalf("[FATAL] migrate: %v", err)
	}
	switch {
	case report.Applied > 0:
		log.Printf("[MIGRATE] applied %d migration(s)", report.Applied)
	case report.Baselined > 0:
		log.Printf("[MIGRATE] baselined %d existing migration(s) (no changes)", report.Baselined)
	default:
		log.Println("[MIGRATE] already up to date")
	}
}
