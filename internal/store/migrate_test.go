package store

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMigrationFilesSortedAndFiltered(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"0002_b.sql", "0001_a.sql", "notes.txt", "0010_c.sql"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("-- x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A directory named like a migration must be skipped, not read as a file.
	if err := os.Mkdir(filepath.Join(dir, "sub.sql"), 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := migrationFiles(dir)
	if err != nil {
		t.Fatalf("migrationFiles: %v", err)
	}
	want := []string{"0001_a.sql", "0002_b.sql", "0010_c.sql"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("file %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestMigrationFilesMissingDir(t *testing.T) {
	if _, err := migrationFiles(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected an error for a missing directory")
	}
}
