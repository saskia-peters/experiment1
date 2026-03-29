package database_test

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"

	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/models"

	_ "modernc.org/sqlite"
)

// newTestDB creates a fresh isolated SQLite database using the real
// InitDatabase (so the schema is always in sync with production). The
// temporary file is cleaned up automatically when the test finishes.
// Do NOT call t.Parallel() in tests that use this helper — they share the
// models.DbFile global, and Go runs intra-package tests sequentially by default.
func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	orig := models.DbFile
	models.DbFile = filepath.Join(t.TempDir(), "test.db")
	t.Cleanup(func() { models.DbFile = orig })

	db, err := database.InitDatabase()
	if err != nil {
		t.Fatalf("newTestDB: InitDatabase: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

// mustInsertParticipants seeds participant rows (header + data) and fatals on error.
func mustInsertParticipants(t *testing.T, db *sql.DB, rows [][]string) {
	t.Helper()
	if err := database.InsertData(db, rows); err != nil {
		t.Fatalf("mustInsertParticipants: %v", err)
	}
}

// mustInsertStations seeds station rows (header + data) and fatals on error.
func mustInsertStations(t *testing.T, db *sql.DB, rows [][]string) {
	t.Helper()
	if err := database.InsertStations(db, rows); err != nil {
		t.Fatalf("mustInsertStations: %v", err)
	}
}

// mustInsertBetreuende seeds betreuende rows (header + data) and fatals on error.
func mustInsertBetreuende(t *testing.T, db *sql.DB, rows [][]string) {
	t.Helper()
	if err := database.InsertBetreuende(db, rows); err != nil {
		t.Fatalf("mustInsertBetreuende: %v", err)
	}
}

// participantRows returns a standard XLSX-style row slice (header + n participants)
// for quick seeding. Each participant gets a unique name like "P1", "P2", etc.
func participantRows(n int) [][]string {
	rows := [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
	}
	for i := 1; i <= n; i++ {
		rows = append(rows, []string{
			fmt.Sprintf("P%d", i), "OV-A", "20", "M", "",
		})
	}
	return rows
}
