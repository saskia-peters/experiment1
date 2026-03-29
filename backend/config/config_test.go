package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"THW-JugendOlympiade/backend/config"
)

// ---------------------------------------------------------------------------
// Default
// ---------------------------------------------------------------------------

func TestDefault_ReturnsExpectedValues(t *testing.T) {
	cfg := config.Default()

	if cfg.Veranstaltung.Name != "THW-JugendOlympiade" {
		t.Errorf("Veranstaltung.Name: want %q, got %q", "THW-JugendOlympiade", cfg.Veranstaltung.Name)
	}
	if cfg.Veranstaltung.Jahr != 2026 {
		t.Errorf("Veranstaltung.Jahr: want 2026, got %d", cfg.Veranstaltung.Jahr)
	}
	if cfg.Gruppen.MaxGroesse != 8 {
		t.Errorf("Gruppen.MaxGroesse: want 8, got %d", cfg.Gruppen.MaxGroesse)
	}
	if cfg.Ergebnisse.MinPunkte != 100 {
		t.Errorf("Ergebnisse.MinPunkte: want 100, got %d", cfg.Ergebnisse.MinPunkte)
	}
	if cfg.Ergebnisse.MaxPunkte != 1200 {
		t.Errorf("Ergebnisse.MaxPunkte: want 1200, got %d", cfg.Ergebnisse.MaxPunkte)
	}
	if cfg.Ausgabe.PDFOrdner != "pdfdocs" {
		t.Errorf("Ausgabe.PDFOrdner: want %q, got %q", "pdfdocs", cfg.Ausgabe.PDFOrdner)
	}
	if cfg.Ausgabe.DBName != "data.db" {
		t.Errorf("Ausgabe.DBName: want %q, got %q", "data.db", cfg.Ausgabe.DBName)
	}
	if cfg.Ausgabe.UrkunderStil != "text" {
		t.Errorf("Ausgabe.UrkunderStil: want %q, got %q", "text", cfg.Ausgabe.UrkunderStil)
	}
}

func TestDefault_PassesValidation(t *testing.T) {
	if err := config.Default().Validate(); err != nil {
		t.Errorf("Default config should be valid, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate
// ---------------------------------------------------------------------------

func TestValidate_ErrorCases(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*config.Config)
		wantErr bool
	}{
		{
			name:    "valid defaults",
			mutate:  func(_ *config.Config) {},
			wantErr: false,
		},
		{
			name:    "MaxGroesse zero",
			mutate:  func(c *config.Config) { c.Gruppen.MaxGroesse = 0 },
			wantErr: true,
		},
		{
			name:    "MaxGroesse negative",
			mutate:  func(c *config.Config) { c.Gruppen.MaxGroesse = -1 },
			wantErr: true,
		},
		{
			name:    "MaxGroesse one is valid",
			mutate:  func(c *config.Config) { c.Gruppen.MaxGroesse = 1 },
			wantErr: false,
		},
		{
			name: "MaxPunkte equal to MinPunkte",
			mutate: func(c *config.Config) {
				c.Ergebnisse.MinPunkte = 100
				c.Ergebnisse.MaxPunkte = 100
			},
			wantErr: true,
		},
		{
			name: "MaxPunkte less than MinPunkte",
			mutate: func(c *config.Config) {
				c.Ergebnisse.MinPunkte = 500
				c.Ergebnisse.MaxPunkte = 100
			},
			wantErr: true,
		},
		{
			name: "MaxPunkte greater than MinPunkte is valid",
			mutate: func(c *config.Config) {
				c.Ergebnisse.MinPunkte = 0
				c.Ergebnisse.MaxPunkte = 1
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.Default()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("expected validation error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("expected no error, got: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateAndSave
// ---------------------------------------------------------------------------

func TestValidateAndSave_RejectsMalformedTOML(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, err := config.ValidateAndSave("this is not valid toml ][{")
	if err == nil {
		t.Fatal("expected error for invalid TOML, got nil")
	}
}

func TestValidateAndSave_RejectsInvalidValues(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	toml := `
[veranstaltung]
name = "Test"
jahr = 2026

[gruppen]
max_groesse = 0

[ergebnisse]
min_punkte = 100
max_punkte = 1200

[ausgabe]
pdf_ordner = "pdfdocs"
db_name = "data.db"
urkunden_stil = "text"
bilder_ordner = "pictures"
`
	_, err := config.ValidateAndSave(toml)
	if err == nil {
		t.Fatal("expected validation error for max_groesse=0, got nil")
	}
}

func TestValidateAndSave_WritesFileAndReturnsConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	toml := `
[veranstaltung]
name = "Mein Event"
jahr = 2025

[gruppen]
max_groesse = 6

[ergebnisse]
min_punkte = 50
max_punkte = 900

[ausgabe]
pdf_ordner = "output"
db_name = "myevent.db"
urkunden_stil = "picture"
bilder_ordner = "photos"
`
	cfg, err := config.ValidateAndSave(toml)
	if err != nil {
		t.Fatalf("ValidateAndSave: %v", err)
	}

	if cfg.Veranstaltung.Name != "Mein Event" {
		t.Errorf("Name: want %q, got %q", "Mein Event", cfg.Veranstaltung.Name)
	}
	if cfg.Gruppen.MaxGroesse != 6 {
		t.Errorf("MaxGroesse: want 6, got %d", cfg.Gruppen.MaxGroesse)
	}
	if cfg.Ausgabe.UrkunderStil != "picture" {
		t.Errorf("UrkunderStil: want %q, got %q", "picture", cfg.Ausgabe.UrkunderStil)
	}

	// The config.toml file should have been created
	if _, err := os.Stat(config.ConfigFile); os.IsNotExist(err) {
		t.Errorf("expected %s to be written, but it doesn't exist", config.ConfigFile)
	}
}

// ---------------------------------------------------------------------------
// LoadOrCreate
// ---------------------------------------------------------------------------

func TestLoadOrCreate_CreatesDefaultConfigWhenFileAbsent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	// No config.toml exists yet
	cfg, err := config.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	// Returns default values
	if cfg.Gruppen.MaxGroesse != 8 {
		t.Errorf("MaxGroesse: want 8, got %d", cfg.Gruppen.MaxGroesse)
	}

	// Creates the file
	if _, statErr := os.Stat(filepath.Join(dir, config.ConfigFile)); os.IsNotExist(statErr) {
		t.Error("expected config.toml to be created")
	}
}

func TestLoadOrCreate_LoadsExistingValidConfig(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	content := `
[veranstaltung]
name = "Loaded Event"
jahr = 2024

[gruppen]
max_groesse = 5

[ergebnisse]
min_punkte = 200
max_punkte = 800

[ausgabe]
pdf_ordner = "pdfdocs"
db_name = "data.db"
urkunden_stil = "text"
bilder_ordner = "pictures"
`
	if err := os.WriteFile(config.ConfigFile, []byte(content), 0644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	cfg, err := config.LoadOrCreate()
	if err != nil {
		t.Fatalf("LoadOrCreate: %v", err)
	}

	if cfg.Veranstaltung.Name != "Loaded Event" {
		t.Errorf("Name: want %q, got %q", "Loaded Event", cfg.Veranstaltung.Name)
	}
	if cfg.Gruppen.MaxGroesse != 5 {
		t.Errorf("MaxGroesse: want 5, got %d", cfg.Gruppen.MaxGroesse)
	}
}

func TestLoadOrCreate_FallsBackToDefaultOnInvalidTOML(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	if err := os.WriteFile(config.ConfigFile, []byte("not valid toml ][{"), 0644); err != nil {
		t.Fatalf("write invalid config: %v", err)
	}

	_, err := config.LoadOrCreate()
	if err == nil {
		t.Error("expected error for invalid TOML, got nil")
	}
}

func TestLoadOrCreate_ErrorOnInvalidValues(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	content := `
[gruppen]
max_groesse = 0

[ergebnisse]
min_punkte = 100
max_punkte = 100
`
	if err := os.WriteFile(config.ConfigFile, []byte(content), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, err := config.LoadOrCreate()
	if err == nil {
		t.Error("expected validation error for max_groesse=0, got nil")
	}
}

// ---------------------------------------------------------------------------
// ReadRaw
// ---------------------------------------------------------------------------

func TestReadRaw_ReturnsFileContent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	content := "# test config\n"
	if err := os.WriteFile(config.ConfigFile, []byte(content), 0644); err != nil {
		t.Fatalf("write config.toml: %v", err)
	}

	raw, err := config.ReadRaw()
	if err != nil {
		t.Fatalf("ReadRaw: %v", err)
	}
	if raw != content {
		t.Errorf("ReadRaw: want %q, got %q", content, raw)
	}
}

func TestReadRaw_ErrorWhenFileAbsent(t *testing.T) {
	dir := t.TempDir()
	chdir(t, dir)

	_, err := config.ReadRaw()
	if err == nil {
		t.Fatal("expected error for missing config.toml, got nil")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// chdir changes the working directory for the duration of the test and
// restores it when the test ends. Config functions operate on config.toml
// relative to the current working directory.
func chdir(t *testing.T, dir string) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("os.Getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("os.Chdir(%s): %v", dir, err)
	}
	t.Cleanup(func() { os.Chdir(orig) })
}
