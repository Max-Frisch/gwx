package integration

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"gwx/internal/store"
)

// migrateTestDB provisions a throwaway database for the migration tests. It
// mirrors the harness bootstrap but leaves applying migrations to the test.
func migrateTestDB(t *testing.T) *sql.DB {
	t.Helper()

	dsn := baseDSN()
	adminDSN, ok := withDatabase(dsn, "postgres")
	if !ok {
		t.Skipf("integration tests need a URL-style DSN (got %q)", dsn)
	}
	dbName := fmt.Sprintf("gwx_migrate_%d_%d", os.Getpid(), rand.Intn(1_000_000))

	admin, err := sql.Open("postgres", adminDSN)
	if err != nil {
		t.Skipf("open postgres: %v", err)
	}
	pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
	if err := admin.PingContext(pingCtx); err != nil {
		pingCancel()
		admin.Close()
		t.Skipf("postgres unavailable at %s: %v", adminDSN, err)
	}
	pingCancel()

	if _, err := admin.Exec(`CREATE DATABASE ` + quoteIdent(dbName)); err != nil {
		admin.Close()
		t.Fatalf("create test database %s: %v", dbName, err)
	}

	testDSN, _ := withDatabase(dsn, dbName)
	db, err := store.NewPostgres(context.Background(), testDSN)
	if err != nil {
		admin.Close()
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() {
		db.Close()
		_, _ = admin.Exec(`DROP DATABASE IF EXISTS ` + quoteIdent(dbName) + ` WITH (FORCE)`)
		admin.Close()
	})
	return db
}

func migrationsDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate migrations")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "migrations")
}

func migrationSQLFiles(t *testing.T, dir string) []string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(dir, "*.sql"))
	if err != nil {
		t.Fatalf("glob migrations: %v", err)
	}
	return files
}

// applyRawMigrations replays every migration file directly, the way gwx applied
// them before the ledger existed. Used to build a "legacy" populated database.
func applyRawMigrations(t *testing.T, db *sql.DB, dir string) {
	t.Helper()
	for _, f := range migrationSQLFiles(t, dir) {
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		if _, err := db.Exec(string(body)); err != nil {
			t.Fatalf("apply %s: %v", f, err)
		}
	}
}

// seedActiveQueueConflict inserts the data the dropped unique indexes forbade:
// several active building AND research orders for one planet/account. A full
// replay of 0001 would now fail on this database.
func seedActiveQueueConflict(t *testing.T, db *sql.DB) {
	t.Helper()
	var universeID string
	if err := db.QueryRow(`
		INSERT INTO universes (code_name, display_name) VALUES ('legacy', 'Legacy') RETURNING id::text
	`).Scan(&universeID); err != nil {
		t.Fatalf("seed universe: %v", err)
	}
	var userID int64
	if err := db.QueryRow(`
		INSERT INTO users (universe_id, username, email, password_hash)
		VALUES ($1, 'legacy', 'legacy@example.com', 'x') RETURNING id
	`, universeID).Scan(&userID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var planetID int64
	if err := db.QueryRow(`
		INSERT INTO planets (universe_id, user_id, galaxy, system, position)
		VALUES ($1, $2, 1, 1, 1) RETURNING id
	`, universeID, userID).Scan(&planetID); err != nil {
		t.Fatalf("seed planet: %v", err)
	}
	for i := 0; i < 2; i++ {
		if _, err := db.Exec(`
			INSERT INTO queues (universe_id, user_id, planet_id, kind, code, target_level, start_at, finish_at)
			VALUES ($1, $2, $3, 'building', 'iron_mine', $4, NOW(), NOW() + INTERVAL '1 hour')
		`, universeID, userID, planetID, i+2); err != nil {
			t.Fatalf("seed building order: %v", err)
		}
		if _, err := db.Exec(`
			INSERT INTO queues (universe_id, user_id, planet_id, kind, code, target_level, start_at, finish_at)
			VALUES ($1, $2, $3, 'research', 'combustion_drive', $4, NOW(), NOW() + INTERVAL '1 hour')
		`, universeID, userID, planetID, i+1); err != nil {
			t.Fatalf("seed research order: %v", err)
		}
	}
}

// TestMigrateLedger proves a fresh database is migrated once, recorded, and then
// skipped on every later run, and that only a pending file is (re)applied.
func TestMigrateLedger(t *testing.T) {
	db := migrateTestDB(t)
	dir := migrationsDir(t)
	ctx := context.Background()

	files := migrationSQLFiles(t, dir)
	if len(files) == 0 {
		t.Fatal("no migration files found")
	}

	report, err := store.Migrate(ctx, db, dir)
	if err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if report.Applied != len(files) {
		t.Errorf("first run applied %d, want %d", report.Applied, len(files))
	}
	if n := queryCount(t, db, `SELECT count(*) FROM schema_migrations`); n != len(files) {
		t.Errorf("ledger has %d rows, want %d", n, len(files))
	}

	again, err := store.Migrate(ctx, db, dir)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if again.Applied != 0 || again.Baselined != 0 {
		t.Errorf("second run applied=%d baselined=%d, want 0/0", again.Applied, again.Baselined)
	}

	// Drop the newest migration from the ledger: only it should be reapplied.
	newest := filepath.Base(files[len(files)-1])
	if _, err := db.Exec(`DELETE FROM schema_migrations WHERE filename = $1`, newest); err != nil {
		t.Fatalf("forget %s: %v", newest, err)
	}
	pending, err := store.Migrate(ctx, db, dir)
	if err != nil {
		t.Fatalf("pending migrate: %v", err)
	}
	if pending.Applied != 1 {
		t.Errorf("pending run applied %d, want 1", pending.Applied)
	}
}

// TestMigrateBaselinesPopulatedDatabase proves the bootstrap path: a database
// that already carries the schema (and legacy data that would break a replay) is
// recorded as applied without re-running the migrations.
func TestMigrateBaselinesPopulatedDatabase(t *testing.T) {
	db := migrateTestDB(t)
	dir := migrationsDir(t)
	ctx := context.Background()

	applyRawMigrations(t, db, dir)
	seedActiveQueueConflict(t, db)

	report, err := store.Migrate(ctx, db, dir)
	if err != nil {
		t.Fatalf("baseline migrate: %v", err)
	}
	if report.Applied != 0 {
		t.Errorf("baseline run applied %d, want 0", report.Applied)
	}
	if want := len(migrationSQLFiles(t, dir)); report.Baselined != want {
		t.Errorf("baseline run baselined %d, want %d", report.Baselined, want)
	}

	// The conflicting rows must still be there and the next run a clean no-op.
	if n := queryCount(t, db, `
		SELECT count(*) FROM queues WHERE kind = 'building' AND status = 'IN_PROGRESS'
	`); n != 2 {
		t.Errorf("active building orders = %d, want 2", n)
	}
	second, err := store.Migrate(ctx, db, dir)
	if err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	if second.Applied != 0 || second.Baselined != 0 {
		t.Errorf("second run applied=%d baselined=%d, want 0/0", second.Applied, second.Baselined)
	}
}

func queryCount(t *testing.T, db *sql.DB, q string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(q).Scan(&n); err != nil {
		t.Fatalf("count query: %v", err)
	}
	return n
}
