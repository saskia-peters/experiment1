package io

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"THW-JugendOlympiade/backend/config"
	"THW-JugendOlympiade/backend/database"

	"github.com/go-pdf/fpdf"
)

const (
	stationColWidthGruppe    = 100.0
	stationColWidthErgebnis  = 45.0
	stationColWidthPunktzahl = 35.0
	stationRowHeight         = 12.0
)

// GenerateStationSheetsPDF creates a PDF with one recording sheet per station.
// Each sheet lists all groups with blank "Ergebnis" and "Punktzahl" columns for
// manual entry during the event. Stations with many groups automatically
// continue on extra pages with a repeated station header and column headers.
func GenerateStationSheetsPDF(db *sql.DB, eventName string, eventYear int, groupNames []string) error {
	if err := ensurePDFDirectory(); err != nil {
		return err
	}

	stations, err := database.GetStationNamesOrdered(db)
	if err != nil {
		return fmt.Errorf("failed to get stations: %w", err)
	}
	if len(stations) == 0 {
		return fmt.Errorf("no stations found to generate station sheets")
	}

	groupIDs, err := database.GetAllGroupIDs(db)
	if err != nil {
		return fmt.Errorf("failed to get group IDs: %w", err)
	}
	if len(groupIDs) == 0 {
		return fmt.Errorf("no groups found to generate station sheets")
	}

	theme := DefaultTheme
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// currentStation and stationFirstPage are captured by the header closure.
	// stationFirstPage is true for the first page of each station and false for
	// any continuation pages (auto page breaks within the same station).
	var currentStation string
	var stationFirstPage bool

	renderColumnHeaders := func() {
		theme.Font(pdf, "B", theme.SizeTableHeader)
		theme.FillColor(pdf, theme.ColorTableHeader)
		theme.TextColor(pdf, theme.ColorText)
		pdf.CellFormat(stationColWidthGruppe, 9, enc("Gruppe"), "1", 0, "C", true, 0, "")
		pdf.CellFormat(stationColWidthErgebnis, 9, enc("Ergebnis"), "1", 0, "C", true, 0, "")
		pdf.CellFormat(stationColWidthPunktzahl, 9, enc("Punktzahl"), "1", 0, "C", true, 0, "")
		pdf.Ln(-1)
	}

	pdf.SetHeaderFunc(func() {
		pdf.SetY(15)

		// Event name + year header
		if eventName != "" {
			theme.Font(pdf, "B", theme.SizeTitle+4)
			theme.TextColor(pdf, theme.ColorText)
			header := enc(eventName)
			if eventYear > 0 {
				header += fmt.Sprintf(" %d", eventYear)
			}
			pdf.CellFormat(0, 14, header, "", 1, "C", false, 0, "")
			pdf.Ln(2)
		}

		// Station name — append "— Fortsetzung" on continuation pages
		stationLabel := enc(currentStation)
		if !stationFirstPage {
			stationLabel += enc(" - Fortsetzung")
		}
		theme.Font(pdf, "B", theme.SizeTitle)
		theme.TextColor(pdf, theme.ColorText)
		pdf.CellFormat(0, 12, stationLabel, "", 1, "C", false, 0, "")

		// Subtitle
		theme.Font(pdf, "", theme.SizeSmall)
		theme.TextColor(pdf, theme.ColorSubtext)
		pdf.CellFormat(0, 6, enc("Stationsbewertungszettel"), "", 1, "C", false, 0, "")
		pdf.Ln(4)

		// Table column headers
		renderColumnHeaders()

		// Mark as continuation for any further auto-break pages within this station
		stationFirstPage = false
	})

	for _, station := range stations {
		currentStation = station.StationName
		stationFirstPage = true
		pdf.AddPage()

		for i, groupID := range groupIDs {
			fill := i%2 == 0

			// Build label: "Gruppe N — ConfiguredName" or just "Gruppe N"
			name := config.GetGroupName(groupID, groupNames)
			fallback := fmt.Sprintf("Gruppe %d", groupID)
			var groupLabel string
			if name == fallback {
				groupLabel = fallback
			} else {
				groupLabel = fmt.Sprintf("Gruppe %d - %s", groupID, name)
			}

			theme.Font(pdf, "", theme.SizeBody)
			theme.FillColor(pdf, theme.ColorTableRowAlt)
			theme.TextColor(pdf, theme.ColorText)
			pdf.CellFormat(stationColWidthGruppe, stationRowHeight, enc(groupLabel), "1", 0, "L", fill, 0, "")
			pdf.CellFormat(stationColWidthErgebnis, stationRowHeight, "", "1", 0, "C", fill, 0, "")
			pdf.CellFormat(stationColWidthPunktzahl, stationRowHeight, "", "1", 0, "C", fill, 0, "")
			pdf.Ln(-1)
		}
	}

	return pdf.OutputFileAndClose(filepath.Join(pdfOutputDir, "Stationsbewertungszettel.pdf"))
}
