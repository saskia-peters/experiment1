package io_test

import (
	"path/filepath"
	"strings"
	"testing"

	backendio "THW-JugendOlympiade/backend/io"
	"THW-JugendOlympiade/backend/models"

	"github.com/xuri/excelize/v2"
)

// ---------------------------------------------------------------------------
// ValidateHeaders
// ---------------------------------------------------------------------------

func TestValidateHeaders(t *testing.T) {
	tests := []struct {
		name     string
		headers  []string
		expected []string
		wantErr  bool
	}{
		{
			name:     "exact match",
			headers:  []string{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			expected: []string{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			wantErr:  false,
		},
		{
			name:     "case insensitive",
			headers:  []string{"name", "ORTSVERBAND", "alter", "geschlecht", "pregroup"},
			expected: []string{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			wantErr:  false,
		},
		{
			name:     "headers with surrounding whitespace",
			headers:  []string{"  Name  ", " Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			expected: []string{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			wantErr:  false,
		},
		{
			name:     "extra columns allowed",
			headers:  []string{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup", "ExtraColumn"},
			expected: []string{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			wantErr:  false,
		},
		{
			name:     "insufficient columns — fewer than expected",
			headers:  []string{"Name", "Ortsverband"},
			expected: []string{"Name", "Ortsverband", "Alter", "Geschlecht"},
			wantErr:  true,
		},
		{
			name:     "empty headers slice",
			headers:  []string{},
			expected: []string{"Name"},
			wantErr:  true,
		},
		{
			name:     "wrong column name",
			headers:  []string{"Name", "City", "Alter", "Geschlecht", "PreGroup"},
			expected: []string{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			wantErr:  true,
		},
		{
			name:     "single expected column — match",
			headers:  []string{"Stationsname"},
			expected: []string{"Stationsname"},
			wantErr:  false,
		},
		{
			name:     "betreuende sheet headers",
			headers:  []string{"Name", "Ortsverband", "Fahrerlaubnis"},
			expected: []string{"Name", "Ortsverband", "Fahrerlaubnis"},
			wantErr:  false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := backendio.ValidateHeaders(tc.headers, tc.expected)
			if tc.wantErr && err == nil {
				t.Error("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ValidateParticipantRow
// ---------------------------------------------------------------------------

func TestValidateParticipantRow(t *testing.T) {
	tests := []struct {
		name    string
		row     []string
		rowNum  int
		wantErr bool
	}{
		{
			name:    "valid complete row",
			row:     []string{"Alice", "Berlin", "25", "W", ""},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "valid row without pregroup column",
			row:     []string{"Alice", "Berlin", "25", "W"},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "empty name — silently accepted (row will be skipped)",
			row:     []string{"", "Berlin", "25", "W"},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "missing ortsverband — warning only, no error",
			row:     []string{"Alice", "", "25", "W"},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "empty age — allowed",
			row:     []string{"Alice", "Berlin", "", "W"},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "unusual gender — warning only, no error",
			row:     []string{"Alice", "Berlin", "25", "X"},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "gender variations — m/w/d accepted",
			row:     []string{"Alice", "Berlin", "25", "d"},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "gender 'männlich' accepted",
			row:     []string{"Alice", "Berlin", "25", "männlich"},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "insufficient columns — less than 4",
			row:     []string{"Alice", "Berlin"},
			rowNum:  2,
			wantErr: true,
		},
		{
			name:    "non-numeric age",
			row:     []string{"Alice", "Berlin", "twenty", "W"},
			rowNum:  2,
			wantErr: true,
		},
		{
			name:    "negative age",
			row:     []string{"Alice", "Berlin", "-1", "W"},
			rowNum:  2,
			wantErr: true,
		},
		{
			name:    "age over 150",
			row:     []string{"Alice", "Berlin", "151", "W"},
			rowNum:  2,
			wantErr: true,
		},
		{
			name:    "age exactly 150 — valid",
			row:     []string{"Alice", "Berlin", "150", "W"},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "age 0 — valid",
			row:     []string{"Alice", "Berlin", "0", "W"},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "valid alphanumeric pregroup",
			row:     []string{"Alice", "Berlin", "25", "W", "Team1"},
			rowNum:  2,
			wantErr: false,
		},
		{
			name:    "pregroup too long (>20 chars)",
			row:     []string{"Alice", "Berlin", "25", "W", "ThisPreGroupNameIsTooLongXXX"},
			rowNum:  2,
			wantErr: true,
		},
		{
			name:    "pregroup with special characters",
			row:     []string{"Alice", "Berlin", "25", "W", "Team-One"},
			rowNum:  2,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := backendio.ValidateParticipantRow(tc.row, tc.rowNum)
			if tc.wantErr && err == nil {
				t.Error("expected an error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// XLSX helpers
// ---------------------------------------------------------------------------

// newXLSX creates an XLSX file at dir/name.xlsx with the given sheets.
// sheets is a map of sheet-name → rows (each row is a []string of cell values).
func newXLSX(t *testing.T, dir, name string, sheets map[string][][]string) string {
	t.Helper()
	f := excelize.NewFile()
	defer f.Close()

	first := true
	for sheet, rows := range sheets {
		if first {
			// The default sheet is "Sheet1"; rename it to the first sheet name.
			f.SetSheetName("Sheet1", sheet)
			first = false
		} else {
			f.NewSheet(sheet)
		}
		for r, row := range rows {
			for c, cell := range row {
				col, _ := excelize.ColumnNumberToName(c + 1)
				f.SetCellValue(sheet, col+string(rune('0'+r+1)), cell)
			}
		}
	}

	path := filepath.Join(dir, name+".xlsx")
	if err := f.SaveAs(path); err != nil {
		t.Fatalf("newXLSX SaveAs %s: %v", path, err)
	}
	return path
}

// ---------------------------------------------------------------------------
// ReadXLSXFile
// ---------------------------------------------------------------------------

func TestReadXLSXFile_ReturnsRowsOnValidFile(t *testing.T) {
	dir := t.TempDir()
	path := newXLSX(t, dir, "data", map[string][][]string{
		models.SheetName: {
			{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			{"Alice", "Berlin", "25", "W", ""},
			{"Bob", "Hamburg", "22", "M", ""},
		},
	})

	rows, err := backendio.ReadXLSXFile(path)
	if err != nil {
		t.Fatalf("ReadXLSXFile: %v", err)
	}
	if len(rows) != 3 { // header + 2 data rows
		t.Errorf("expected 3 rows (header + 2 data), got %d", len(rows))
	}
}

func TestReadXLSXFile_ErrorWhenFileAbsent(t *testing.T) {
	_, err := backendio.ReadXLSXFile(filepath.Join(t.TempDir(), "nonexistent.xlsx"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestReadXLSXFile_ErrorWhenSheetMissing(t *testing.T) {
	dir := t.TempDir()
	// Create an XLSX with a different sheet name — Teilnehmende sheet is absent.
	path := newXLSX(t, dir, "no_sheet", map[string][][]string{
		"WrongSheet": {
			{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			{"Alice", "Berlin", "25", "W", ""},
		},
	})

	_, err := backendio.ReadXLSXFile(path)
	if err == nil {
		t.Fatal("expected error when Teilnehmende sheet is missing, got nil")
	}
}

func TestReadXLSXFile_ErrorWhenHeadersWrong(t *testing.T) {
	dir := t.TempDir()
	path := newXLSX(t, dir, "bad_header", map[string][][]string{
		models.SheetName: {
			{"Vorname", "Stadt", "Alter", "Geschlecht", "PreGroup"},
			{"Alice", "Berlin", "25", "W", ""},
		},
	})

	_, err := backendio.ReadXLSXFile(path)
	if err == nil {
		t.Fatal("expected error for wrong headers, got nil")
	}
}

func TestReadXLSXFile_ErrorWhenNoDataRows(t *testing.T) {
	dir := t.TempDir()
	// Only a header row — no participants.
	path := newXLSX(t, dir, "header_only", map[string][][]string{
		models.SheetName: {
			{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		},
	})

	_, err := backendio.ReadXLSXFile(path)
	if err == nil {
		t.Fatal("expected error for header-only sheet, got nil")
	}
}

func TestReadXLSXFile_ErrorWhenRowHasInvalidAge(t *testing.T) {
	dir := t.TempDir()
	path := newXLSX(t, dir, "bad_age", map[string][][]string{
		models.SheetName: {
			{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			{"Alice", "Berlin", "not-a-number", "W", ""},
		},
	})

	_, err := backendio.ReadXLSXFile(path)
	if err == nil {
		t.Fatal("expected error for invalid age, got nil")
	}
}

// ---------------------------------------------------------------------------
// ReadBetreuendeFromXLSX
// ---------------------------------------------------------------------------

func TestReadBetreuendeFromXLSX_ReturnsRowsOnValidFile(t *testing.T) {
	dir := t.TempDir()
	path := newXLSX(t, dir, "data", map[string][][]string{
		models.BetreuendeSheetName: {
			{"Name", "Ortsverband", "Fahrerlaubnis"},
			{"Trainer A", "Berlin", "ja"},
			{"Trainer B", "Hamburg", "nein"},
		},
	})

	rows, err := backendio.ReadBetreuendeFromXLSX(path)
	if err != nil {
		t.Fatalf("ReadBetreuendeFromXLSX: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 rows (header + 2 data), got %d", len(rows))
	}
}

func TestReadBetreuendeFromXLSX_ErrorWhenFileAbsent(t *testing.T) {
	_, err := backendio.ReadBetreuendeFromXLSX(filepath.Join(t.TempDir(), "missing.xlsx"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestReadBetreuendeFromXLSX_ReturnsEmptyWhenSheetAbsent(t *testing.T) {
	dir := t.TempDir()
	// File exists but has no Betreuende sheet — that's acceptable.
	path := newXLSX(t, dir, "no_betreuende", map[string][][]string{
		models.SheetName: {
			{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			{"Alice", "Berlin", "25", "W", ""},
		},
	})

	rows, err := backendio.ReadBetreuendeFromXLSX(path)
	if err != nil {
		t.Fatalf("expected no error when Betreuende sheet is absent, got: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("expected empty slice, got %d rows", len(rows))
	}
}

func TestReadBetreuendeFromXLSX_ErrorOnInvalidFahrerlaubnis(t *testing.T) {
	dir := t.TempDir()
	path := newXLSX(t, dir, "bad_f", map[string][][]string{
		models.BetreuendeSheetName: {
			{"Name", "Ortsverband", "Fahrerlaubnis"},
			{"Trainer", "Berlin", "vielleicht"}, // invalid value
		},
	})

	_, err := backendio.ReadBetreuendeFromXLSX(path)
	if err == nil {
		t.Fatal("expected error for invalid Fahrerlaubnis value, got nil")
	}
}

// ---------------------------------------------------------------------------
// ReadStationsFromXLSX
// ---------------------------------------------------------------------------

func TestReadStationsFromXLSX_ReturnsRowsOnValidFile(t *testing.T) {
	dir := t.TempDir()
	path := newXLSX(t, dir, "data", map[string][][]string{
		models.StationsSheetName: {
			{"Stationsname"},
			{"Bogenschießen"},
			{"Sanitätsdienst"},
		},
	})

	rows, err := backendio.ReadStationsFromXLSX(path)
	if err != nil {
		t.Fatalf("ReadStationsFromXLSX: %v", err)
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 rows (header + 2 data), got %d", len(rows))
	}
}

func TestReadStationsFromXLSX_ErrorWhenFileAbsent(t *testing.T) {
	_, err := backendio.ReadStationsFromXLSX(filepath.Join(t.TempDir(), "missing.xlsx"))
	if err == nil {
		t.Fatal("expected error for missing file, got nil")
	}
}

func TestReadStationsFromXLSX_ReturnsEmptyWhenSheetAbsent(t *testing.T) {
	dir := t.TempDir()
	// File with only a Teilnehmende sheet — Stationen sheet is absent.
	// Since stations are now required, this must return an error.
	path := newXLSX(t, dir, "no_stationen", map[string][][]string{
		models.SheetName: {
			{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
			{"Alice", "Berlin", "25", "W", ""},
		},
	})

	_, err := backendio.ReadStationsFromXLSX(path)
	if err == nil {
		t.Fatal("expected error when Stationen sheet is absent, got nil")
	}
	if !strings.Contains(err.Error(), "Keine Stationen") {
		t.Errorf("expected error about missing stations, got: %v", err)
	}
}
