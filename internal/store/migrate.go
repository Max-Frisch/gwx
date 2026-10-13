package store

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// migrationLockKey is an arbitrary fixed key for the migrator's session-level
// advisory lock. Every gwx migrator that runs against the same database uses it
// so concurrent runs serialize instead of racing.
const migrationLockKey = int64(0x6777786D)

// MigrationReport summarizes what a Migrate run did.
type MigrationReport struct {
	// Applied is how many pending migration files were executed.
	Applied int
	// Baselined is how many files were recorded without executing them, because
	// the database predates the schema_migrations ledger (see Migrate).
	Baselined int
}

// Migrate brings a database up to date with the .sql files in dir, applying only
// the ones not already recorded in schema_migrations, in filename order, each in
// its own transaction.
//
// It is safe to run repeatedly and concurrently (a Postgres advisory lock
// serializes runs). Because migrations are otherwise replayed on every call, the
// ledger replaces the old "run every file, rely on IF NOT EXISTS" approach that
// broke once a later migration dropped an index an earlier one created.
//
// Bootstrapping: the first run on a database that already has the gwx schema
// (the universes table exists) but no ledger records every file as applied
// without executing it. Without that baseline, replaying 0001 would try to
// recreate indexes that later migrations deliberately dropped, which fails on a
// populated database.
func Migrate(ctx context.Context, db *sql.DB, dir string) (MigrationReport, error) {
	var report MigrationReport

	files, err := migrationFiles(dir)
	if err != nil {
		return report, err
	}

	conn, err := db.Conn(ctx)
	if err != nil {
		return report, err
	}
	defer conn.Close()

	if _, err := conn.ExecContext(ctx, `SELECT pg_advisory_lock($1)`, migrationLockKey); err != nil {
		return report, fmt.Errorf("acquire migration lock: %w", err)
	}
	defer func() {
		_, _ = conn.ExecContext(context.Background(), `SELECT pg_advisory_unlock($1)`, migrationLockKey)
	}()

	var ledgerExisted bool
	if err := conn.QueryRowContext(ctx,
		`SELECT to_regclass('schema_migrations') IS NOT NULL`).Scan(&ledgerExisted); err != nil {
		return report, err
	}
	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename   TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)
	`); err != nil {
		return report, fmt.Errorf("create schema_migrations: %w", err)
	}

	// First contact with a database that already carries the schema: trust that
	// its migrations were applied the old way and just record them.
	if !ledgerExisted {
		var initialized bool
		if err := conn.QueryRowContext(ctx,
			`SELECT to_regclass('universes') IS NOT NULL`).Scan(&initialized); err != nil {
			return report, err
		}
		if initialized {
			for _, name := range files {
				if _, err := conn.ExecContext(ctx, `
					INSERT INTO schema_migrations (filename) VALUES ($1) ON CONFLICT DO NOTHING
				`, name); err != nil {
					return report, fmt.Errorf("baseline %s: %w", name, err)
				}
				report.Baselined++
			}
			return report, nil
		}
	}

	for _, name := range files {
		var exists bool
		if err := conn.QueryRowContext(ctx,
			`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE filename = $1)`, name).Scan(&exists); err != nil {
			return report, err
		}
		if exists {
			continue
		}
		body, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return report, fmt.Errorf("read %s: %w", name, err)
		}
		tx, err := conn.BeginTx(ctx, nil)
		if err != nil {
			return report, err
		}
		if _, err := tx.ExecContext(ctx, string(body)); err != nil {
			tx.Rollback()
			return report, fmt.Errorf("apply %s: %w", name, err)
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO schema_migrations (filename) VALUES ($1)`, name); err != nil {
			tx.Rollback()
			return report, fmt.Errorf("record %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return report, fmt.Errorf("commit %s: %w", name, err)
		}
		report.Applied++
	}
	return report, nil
}

// migrationFiles returns the .sql files in dir, sorted by filename.
func migrationFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("read migrations dir: %w", err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		names = append(names, e.Name())
	}
	sort.Strings(names)
	return names, nil
}
