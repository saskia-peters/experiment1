package database_test

import (
	"path/filepath"
	"testing"

	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/models"
)

// ---------------------------------------------------------------------------
// TrimSpace
// ---------------------------------------------------------------------------

func TestTrimSpace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain string", "hello", "hello"},
		{"leading spaces", "  hello", "hello"},
		{"trailing spaces", "hello  ", "hello"},
		{"both sides", "  hello  ", "hello"},
		{"tabs and newlines", "\t\nhello\n\t", "hello"},
		{"empty string", "", ""},
		{"only whitespace", "  ", ""},
		{"internal spaces preserved", "  two  words  ", "two  words"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := database.TrimSpace(tc.input)
			if got != tc.want {
				t.Errorf("TrimSpace(%q) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// InitDatabase
// ---------------------------------------------------------------------------

func TestInitDatabase_CreatesRequiredTables(t *testing.T) {
	db := newTestDB(t)

	tables := []string{
		"teilnehmende",
		"gruppe",
		"stations",
		"group_station_scores",
		"betreuende",
		"gruppe_betreuende",
	}
	for _, tbl := range tables {
		var count int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?", tbl,
		).Scan(&count); err != nil {
			t.Fatalf("sqlite_master query for table %s: %v", tbl, err)
		}
		if count != 1 {
			t.Errorf("expected table %q to exist", tbl)
		}
	}
}

func TestInitDatabase_CreatesRequiredIndexes(t *testing.T) {
	db := newTestDB(t)

	indexes := []string{
		"idx_gruppe_group_id",
		"idx_gruppe_teilnehmer_id",
		"idx_scores_group_id",
		"idx_scores_station_id",
	}
	for _, idx := range indexes {
		var count int
		if err := db.QueryRow(
			"SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name=?", idx,
		).Scan(&count); err != nil {
			t.Fatalf("sqlite_master query for index %s: %v", idx, err)
		}
		if count != 1 {
			t.Errorf("expected index %q to exist", idx)
		}
	}
}

func TestInitDatabase_WipesExistingData(t *testing.T) {
	orig := models.DbFile
	models.DbFile = filepath.Join(t.TempDir(), "wipe_test.db")
	t.Cleanup(func() { models.DbFile = orig })

	// First init + seed
	db1, err := database.InitDatabase()
	if err != nil {
		t.Fatalf("first InitDatabase: %v", err)
	}
	if _, err := db1.Exec("INSERT INTO teilnehmende (teilnehmer_id, name) VALUES (1, 'Test')"); err != nil {
		t.Fatalf("seed insert: %v", err)
	}
	db1.Close()

	// Second init on same file must wipe prior data
	db2, err := database.InitDatabase()
	if err != nil {
		t.Fatalf("second InitDatabase: %v", err)
	}
	defer db2.Close()

	var count int
	db2.QueryRow("SELECT COUNT(*) FROM teilnehmende").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 rows after re-initialisation, got %d", count)
	}
}

func TestInitDatabase_EnablesForeignKeys(t *testing.T) {
	db := newTestDB(t)

	var fk int
	if err := db.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("expected foreign_keys=1, got %d", fk)
	}
}

// ---------------------------------------------------------------------------
// OpenExistingDB
// ---------------------------------------------------------------------------

func TestOpenExistingDB_ErrorWhenFileAbsent(t *testing.T) {
	orig := models.DbFile
	models.DbFile = filepath.Join(t.TempDir(), "nonexistent.db")
	t.Cleanup(func() { models.DbFile = orig })

	_, err := database.OpenExistingDB()
	if err == nil {
		t.Fatal("expected an error for a missing database file, got nil")
	}
}

func TestOpenExistingDB_SuccessAfterInit(t *testing.T) {
	orig := models.DbFile
	models.DbFile = filepath.Join(t.TempDir(), "existing.db")
	t.Cleanup(func() { models.DbFile = orig })

	// Create the file with InitDatabase first
	db1, err := database.InitDatabase()
	if err != nil {
		t.Fatalf("InitDatabase: %v", err)
	}
	db1.Close()

	// OpenExistingDB must succeed
	db2, err := database.OpenExistingDB()
	if err != nil {
		t.Fatalf("OpenExistingDB: %v", err)
	}
	defer db2.Close()

	// Must have foreign keys enabled
	var fk int
	if err := db2.QueryRow("PRAGMA foreign_keys").Scan(&fk); err != nil {
		t.Fatalf("PRAGMA foreign_keys: %v", err)
	}
	if fk != 1 {
		t.Errorf("OpenExistingDB: expected foreign_keys=1, got %d", fk)
	}
}
