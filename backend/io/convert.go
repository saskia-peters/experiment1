package io

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"THW-JugendOlympiade/backend/models"

	"github.com/xuri/excelize/v2"
)

// ----------------------------------------------------------------------------
// Typed intermediate data structures for the conversion pipeline.
// TransformMasterExcel fills these from the raw source data; WriteMasterExcel
// serialises them into the exact format expected by LoadFile / "Excel einlesen".
// ----------------------------------------------------------------------------

// ConvertedTeilnehmender is one row for the "Teilnehmende" sheet.
// Columns: Name | Ortsverband | Alter | Geschlecht | PreGroup
type ConvertedTeilnehmender struct {
	Name        string
	Ortsverband string
	Alter       int    // 0 = unknown / will be written as empty string
	Geschlecht  string // "m" | "w" | "d"
	PreGroup    string // optional
}

// ConvertedBetreuender is one row for the "Betreuende" sheet.
// Columns: Name | Ortsverband | Fahrerlaubnis
type ConvertedBetreuender struct {
	Name          string
	Ortsverband   string
	Fahrerlaubnis bool // written as "ja" / "nein"
}

// ConvertedStation is one row for the "Stationen" sheet.
// Columns: Name
type ConvertedStation struct {
	Name string
}

// ConvertedFahrzeug is one row for the "Fahrzeuge" sheet.
// Columns: Bezeichnung | Ortsverband | Funkrufname | Fahrer | Sitzplaetze
type ConvertedFahrzeug struct {
	Bezeichnung string
	Ortsverband string
	Funkrufname string
	Fahrer      string
	Sitzplaetze int // >= 1
}

// ConvertedData is the output of TransformMasterExcel.
// It maps 1-to-1 onto the four sheets that LoadFile / "Excel einlesen" expects.
type ConvertedData struct {
	Teilnehmende []ConvertedTeilnehmender
	Betreuende   []ConvertedBetreuender
	Stationen    []ConvertedStation
	Fahrzeuge    []ConvertedFahrzeug
}

// MasterEvent selects which sub-event to extract from the Master Excel.
type MasterEvent string

const (
	EventJugend MasterEvent = "Jugend"
	EventMini   MasterEvent = "Mini"
)

// ----------------------------------------------------------------------------
// Pipeline steps
// ----------------------------------------------------------------------------

// MasterExcelData holds the raw sheet contents of a source Excel file.
type MasterExcelData struct {
	// Sheets maps sheet name → rows (first row is the header).
	Sheets map[string][][]string
	// SheetOrder preserves the original sheet sequence.
	SheetOrder []string
}

// ReadMasterExcel reads every sheet from the given xlsx file into a
// MasterExcelData.  No format validation is performed here — that is the
// responsibility of TransformMasterExcel.
func ReadMasterExcel(filePath string) (*MasterExcelData, error) {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("Datei '%s' nicht gefunden", filePath)
	}

	f, err := excelize.OpenFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("Excel-Datei konnte nicht geöffnet werden: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("ReadMasterExcel: close error: %v", err)
		}
	}()

	sheetList := f.GetSheetList()
	data := &MasterExcelData{
		Sheets:     make(map[string][][]string, len(sheetList)),
		SheetOrder: sheetList,
	}

	for _, sheet := range sheetList {
		rows, err := f.GetRows(sheet)
		if err != nil {
			return nil, fmt.Errorf("Tabellenblatt '%s' konnte nicht gelesen werden: %w", sheet, err)
		}
		data.Sheets[sheet] = rows
	}

	return data, nil
}

// TransformMasterExcel converts the raw source data into a ConvertedData that
// WriteMasterExcel will serialise into the standard import format.
//
// The event parameter controls which rows are selected:
//
//	EventJugend:
//	  Betreuende  → Betreuende="x" AND Mini=blank
//	  Teilnehmende → JuHe="x"
//
//	EventMini:
//	  Betreuende  → Betreuende="x" AND Mini="x"
//	  Teilnehmende → Mini="x" AND Betreuende=blank
//
// Source column "Fahrerlaubnis": any non-empty value other than "/" → true.
//
// Source layout:
//
//	Sheet "Teilnehmende": Vorname | Name | Betreuende | JuHe | Mini |
//	                       Alter | Ortsverband | Fahrerlaubnis | Geschlecht
//	Sheet "Fahrzeuge":    Fahrzeug | Ortsverband | Funkrufname | Fahrer |
//	                       Anzahl Plätze incl. Fahrende
func TransformMasterExcel(src *MasterExcelData, event MasterEvent) *ConvertedData {
	out := &ConvertedData{
		Teilnehmende: []ConvertedTeilnehmender{},
		Betreuende:   []ConvertedBetreuender{},
		Stationen:    []ConvertedStation{},
		Fahrzeuge:    []ConvertedFahrzeug{},
	}

	// --- Source "Teilnehmende" sheet ---
	srcTeiln, ok := src.Sheets["Teilnehmende"]
	if !ok || len(srcTeiln) < 2 {
		log.Printf("TransformMasterExcel: sheet 'Teilnehmende' not found or empty")
	} else {
		idx := buildHeaderIndex(srcTeiln[0])
		for rowNum, row := range srcTeiln[1:] {
			vorname := getCol(row, idx, "Vorname")
			nachname := getCol(row, idx, "Name")
			fullName := strings.TrimSpace(vorname + " " + nachname)
			if fullName == "" {
				continue
			}

			ortsverband := getCol(row, idx, "Ortsverband")
			alterStr := getCol(row, idx, "Alter")
			alter := 0
			if alterStr != "" {
				if v, err := strconv.Atoi(alterStr); err == nil && v > 0 {
					alter = v
				} else {
					log.Printf("TransformMasterExcel: Teilnehmende row %d: invalid Alter %q", rowNum+2, alterStr)
				}
			}

			isBetreuende := strings.EqualFold(getCol(row, idx, "Betreuende"), "x")
			isMini := strings.EqualFold(getCol(row, idx, "Mini"), "x")
			isJuHe := strings.EqualFold(getCol(row, idx, "JuHe"), "x")

			// Fahrerlaubnis: non-empty value that is not "/" means licensed
			fahrVal := getCol(row, idx, "Fahrerlaubnis")
			fahrerlaubnis := fahrVal != "" && fahrVal != "/"

			geschlecht := getCol(row, idx, "Geschlecht")

			switch event {
			case EventJugend:
				// Betreuende: marked as Betreuende and NOT Mini
				if isBetreuende && !isMini {
					out.Betreuende = append(out.Betreuende, ConvertedBetreuender{
						Name:          fullName,
						Ortsverband:   ortsverband,
						Fahrerlaubnis: fahrerlaubnis,
					})
				}
				// Teilnehmende: JuHe marked
				if isJuHe {
					out.Teilnehmende = append(out.Teilnehmende, ConvertedTeilnehmender{
						Name:        fullName,
						Ortsverband: ortsverband,
						Alter:       alter,
						Geschlecht:  geschlecht,
						PreGroup:    "",
					})
				}
			case EventMini:
				// Betreuende: marked as Betreuende AND Mini
				if isBetreuende && isMini {
					out.Betreuende = append(out.Betreuende, ConvertedBetreuender{
						Name:          fullName,
						Ortsverband:   ortsverband,
						Fahrerlaubnis: fahrerlaubnis,
					})
				}
				// Teilnehmende: Mini marked and NOT Betreuende
				if isMini && !isBetreuende {
					out.Teilnehmende = append(out.Teilnehmende, ConvertedTeilnehmender{
						Name:        fullName,
						Ortsverband: ortsverband,
						Alter:       alter,
						Geschlecht:  geschlecht,
						PreGroup:    "",
					})
				}
			}
		}
	}

	// --- Source "Fahrzeuge" sheet (Jugend only) ---
	if event == EventJugend {
		srcFahr, ok := src.Sheets["Fahrzeuge"]
		if !ok || len(srcFahr) < 2 {
			log.Printf("TransformMasterExcel: sheet 'Fahrzeuge' not found or empty")
		} else {
			idx := buildHeaderIndex(srcFahr[0])
			for rowNum, row := range srcFahr[1:] {
				bezeichnung := getCol(row, idx, "Fahrzeug")
				if bezeichnung == "" {
					continue
				}
				sitzStr := getCol(row, idx, "Anzahl Plätze incl. Fahrende")
				sitze := 0
				if sitzStr != "" {
					if v, err := strconv.Atoi(sitzStr); err == nil && v > 0 {
						sitze = v
					} else {
						log.Printf("TransformMasterExcel: Fahrzeuge row %d: invalid Sitzplaetze %q", rowNum+2, sitzStr)
					}
				}
				out.Fahrzeuge = append(out.Fahrzeuge, ConvertedFahrzeug{
					Bezeichnung: bezeichnung,
					Ortsverband: getCol(row, idx, "Ortsverband"),
					Funkrufname: getCol(row, idx, "Funkrufname"),
					Fahrer:      getCol(row, idx, "Fahrer"),
					Sitzplaetze: sitze,
				})
			}
		}
	}

	return out
}

// buildHeaderIndex builds a case-insensitive, trimmed name → column-index map
// from a header row.
func buildHeaderIndex(headers []string) map[string]int {
	idx := make(map[string]int, len(headers))
	for i, h := range headers {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	return idx
}

// getCol retrieves and trims the cell value for the given column name
// (case-insensitive) from a data row.  Returns "" if the column is not
// present in the header or the row is too short.
func getCol(row []string, idx map[string]int, col string) string {
	i, ok := idx[strings.ToLower(strings.TrimSpace(col))]
	if !ok || i >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[i])
}

// WriteMasterExcel writes a ConvertedData to destPath as an xlsx with the
// exact four-sheet layout expected by "Excel einlesen" / LoadFile:
//
//	Teilnehmende  — Name | Ortsverband | Alter | Geschlecht | PreGroup
//	Betreuende    — Name | Ortsverband | Fahrerlaubnis
//	Stationen     — Name
//	Fahrzeuge     — Bezeichnung | Ortsverband | Funkrufname | Fahrer | Sitzplaetze
func WriteMasterExcel(destPath string, data *ConvertedData) error {
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("WriteMasterExcel: close error: %v", err)
		}
	}()

	// excelize creates a default "Sheet1"; rename it to the first sheet we write.
	if err := f.SetSheetName("Sheet1", models.SheetName); err != nil {
		return fmt.Errorf("Tabellenblatt '%s' konnte nicht erstellt werden: %w", models.SheetName, err)
	}

	// --- 1. Teilnehmende ---
	teilnHeaders := []interface{}{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}
	if err := writeRow(f, models.SheetName, 1, teilnHeaders); err != nil {
		return err
	}
	for i, t := range data.Teilnehmende {
		alterStr := ""
		if t.Alter > 0 {
			alterStr = strconv.Itoa(t.Alter)
		}
		row := []interface{}{t.Name, t.Ortsverband, alterStr, t.Geschlecht, t.PreGroup}
		if err := writeRow(f, models.SheetName, i+2, row); err != nil {
			return err
		}
	}

	// --- 2. Betreuende ---
	if _, err := f.NewSheet(models.BetreuendeSheetName); err != nil {
		return fmt.Errorf("Tabellenblatt '%s' konnte nicht erstellt werden: %w", models.BetreuendeSheetName, err)
	}
	betHeaders := []interface{}{"Name", "Ortsverband", "Fahrerlaubnis"}
	if err := writeRow(f, models.BetreuendeSheetName, 1, betHeaders); err != nil {
		return err
	}
	for i, b := range data.Betreuende {
		fahrStr := "nein"
		if b.Fahrerlaubnis {
			fahrStr = "ja"
		}
		row := []interface{}{b.Name, b.Ortsverband, fahrStr}
		if err := writeRow(f, models.BetreuendeSheetName, i+2, row); err != nil {
			return err
		}
	}

	// --- 3. Stationen ---
	if _, err := f.NewSheet(models.StationsSheetName); err != nil {
		return fmt.Errorf("Tabellenblatt '%s' konnte nicht erstellt werden: %w", models.StationsSheetName, err)
	}
	if err := writeRow(f, models.StationsSheetName, 1, []interface{}{"Name"}); err != nil {
		return err
	}
	for i, s := range data.Stationen {
		if err := writeRow(f, models.StationsSheetName, i+2, []interface{}{s.Name}); err != nil {
			return err
		}
	}

	// --- 4. Fahrzeuge ---
	if _, err := f.NewSheet(models.FahrzeugeSheetName); err != nil {
		return fmt.Errorf("Tabellenblatt '%s' konnte nicht erstellt werden: %w", models.FahrzeugeSheetName, err)
	}
	fahHeaders := []interface{}{"Bezeichnung", "Ortsverband", "Funkrufname", "Fahrer", "Sitzplaetze"}
	if err := writeRow(f, models.FahrzeugeSheetName, 1, fahHeaders); err != nil {
		return err
	}
	for i, v := range data.Fahrzeuge {
		row := []interface{}{v.Bezeichnung, v.Ortsverband, v.Funkrufname, v.Fahrer, strconv.Itoa(v.Sitzplaetze)}
		if err := writeRow(f, models.FahrzeugeSheetName, i+2, row); err != nil {
			return err
		}
	}

	if err := f.SaveAs(destPath); err != nil {
		return fmt.Errorf("Datei konnte nicht gespeichert werden: %w", err)
	}
	return nil
}

// writeRow writes a slice of values into consecutive cells of the given sheet
// starting at (col=1, rowIdx).
func writeRow(f *excelize.File, sheet string, rowIdx int, values []interface{}) error {
	for col, val := range values {
		cellRef, err := excelize.CoordinatesToCellName(col+1, rowIdx)
		if err != nil {
			return fmt.Errorf("Zellreferenz konnte nicht bestimmt werden: %w", err)
		}
		if err := f.SetCellValue(sheet, cellRef, val); err != nil {
			return fmt.Errorf("Zellwert konnte nicht gesetzt werden: %w", err)
		}
	}
	return nil
}
