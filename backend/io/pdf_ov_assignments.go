package io

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"THW-JugendOlympiade/backend/config"
	"THW-JugendOlympiade/backend/database"

	"github.com/go-pdf/fpdf"
)

// ovPersonRow is an intermediate row used while building the OV assignment tables.
type ovPersonRow struct {
	Name     string
	GroupID  int
	Fahrzeug string // display string for the group's vehicle(s)
	IsFahrer bool   // true when this betreuende is listed as a vehicle driver
}

// ovSection holds the betreuende and teilnehmende lists for one Ortsverband.
type ovSection struct {
	Betreuende   []ovPersonRow
	Teilnehmende []ovPersonRow
}

// GenerateOVAssignmentsPDF creates a PDF with one page per Ortsverband.
// Each page contains two tables: one for Betreuende and one for Teilnehmende,
// showing the assigned group and vehicle(s) for each person.
func GenerateOVAssignmentsPDF(db *sql.DB, eventName string, eventYear int, groupNames []string) error {
	if err := ensurePDFDirectory(); err != nil {
		return err
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		return fmt.Errorf("failed to get groups: %w", err)
	}
	if len(groups) == 0 {
		return fmt.Errorf("no groups found to generate OV assignments report")
	}

	// Build per-OV maps from the groups data.
	ovMap := make(map[string]*ovSection)

	for _, group := range groups {
		// Build a display string for all vehicles in this group.
		var parts []string
		for _, f := range group.Fahrzeuge {
			display := f.Bezeichnung
			if f.Funkrufname != "" {
				display += " (" + f.Funkrufname + ")"
			}
			parts = append(parts, display)
		}
		fahrzeugStr := strings.Join(parts, ", ")

		for _, b := range group.Betreuende {
			ov := b.Ortsverband
			if _, ok := ovMap[ov]; !ok {
				ovMap[ov] = &ovSection{}
			}
			// Check whether this person is listed as a driver on any group vehicle.
			isFahrer := false
			for _, f := range group.Fahrzeuge {
				if f.FahrerName == b.Name {
					isFahrer = true
					break
				}
			}
			ovMap[ov].Betreuende = append(ovMap[ov].Betreuende, ovPersonRow{
				Name:     b.Name,
				GroupID:  group.GroupID,
				Fahrzeug: fahrzeugStr,
				IsFahrer: isFahrer,
			})
		}
		for _, t := range group.Teilnehmende {
			ov := t.Ortsverband
			if _, ok := ovMap[ov]; !ok {
				ovMap[ov] = &ovSection{}
			}
			ovMap[ov].Teilnehmende = append(ovMap[ov].Teilnehmende, ovPersonRow{
				Name:     t.Name,
				GroupID:  group.GroupID,
				Fahrzeug: fahrzeugStr,
			})
		}
	}

	// Sort OV names alphabetically.
	ovNames := make([]string, 0, len(ovMap))
	for ov := range ovMap {
		ovNames = append(ovNames, ov)
	}
	sort.Strings(ovNames)

	// Detect whether any group has vehicles assigned so we can hide the
	// Fahrzeug/Fhr. columns when running in Klassisch mode (no vehicles).
	hasVehicles := false
	for _, g := range groups {
		if len(g.Fahrzeuge) > 0 {
			hasVehicles = true
			break
		}
	}

	theme := DefaultTheme
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(false, 0)

	// Column widths for Betreuende.
	// With vehicles:    Name 50 + Gruppe 50 + Fahrzeug 62 + Fhr. 18 = 180mm
	// Without vehicles: Name 90 + Gruppe 90 = 180mm (same as Teilnehmende)
	var colWB []float64
	if hasVehicles {
		colWB = []float64{50, 50, 62, 18}
	} else {
		colWB = []float64{90, 90}
	}
	// Column widths for Teilnehmende: Name 90 + Gruppe 90 = 180mm
	colWT := []float64{90, 90}

	rowH := 9.0

	// fitCell renders text in a cell, automatically shrinking the font when the
	// text is too wide to fit, then restores the original size.
	fitCell := func(w, h float64, text, border, align string, fill bool, baseSize float64) {
		sz := baseSize
		for sz >= 6 && pdf.GetStringWidth(text) > w-2 {
			sz -= 0.5
			pdf.SetFontSize(sz)
		}
		pdf.CellFormat(w, h, text, border, 0, align, fill, 0, "")
		pdf.SetFontSize(baseSize)
	}

	renderBetreuendeHeader := func() {
		theme.Font(pdf, "B", theme.SizeTableHeader)
		theme.FillColor(pdf, theme.ColorTableHeader)
		theme.TextColor(pdf, theme.ColorText)
		if hasVehicles {
			for i, h := range []string{"Betreuende", "Gruppe", "Fahrzeug", "Fhr."} {
				pdf.CellFormat(colWB[i], 10, enc(h), "1", 0, "C", true, 0, "")
			}
		} else {
			for i, h := range []string{"Betreuende", "Gruppe"} {
				pdf.CellFormat(colWB[i], 10, enc(h), "1", 0, "C", true, 0, "")
			}
		}
		pdf.Ln(-1)
	}

	renderBetreuendeRow := func(row ovPersonRow, fill bool) {
		theme.Font(pdf, "", theme.SizeBody)
		theme.FillColor(pdf, theme.ColorTableRowAlt)
		groupLabel := fmt.Sprintf("Gruppe %d - %s", row.GroupID, config.GetGroupName(row.GroupID, groupNames))
		fitCell(colWB[0], rowH, enc(row.Name), "1", "L", fill, theme.SizeBody)
		fitCell(colWB[1], rowH, enc(groupLabel), "1", "C", fill, theme.SizeBody)
		if hasVehicles {
			fitCell(colWB[2], rowH, enc(row.Fahrzeug), "1", "L", fill, theme.SizeBody)
			check := ""
			if row.IsFahrer {
				check = "X"
			}
			pdf.CellFormat(colWB[3], rowH, check, "1", 0, "C", fill, 0, "")
		}
		pdf.Ln(-1)
	}

	renderTeilnehmendeHeader := func() {
		theme.Font(pdf, "B", theme.SizeTableHeader)
		theme.FillColor(pdf, theme.ColorTableHeader)
		theme.TextColor(pdf, theme.ColorText)
		for i, h := range []string{"Teilnehmende", "Gruppe"} {
			pdf.CellFormat(colWT[i], 10, enc(h), "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}

	renderTeilnehmendeRow := func(row ovPersonRow, fill bool) {
		theme.Font(pdf, "", theme.SizeBody)
		theme.FillColor(pdf, theme.ColorTableRowAlt)
		groupLabel := fmt.Sprintf("Gruppe %d - %s", row.GroupID, config.GetGroupName(row.GroupID, groupNames))
		fitCell(colWT[0], rowH, enc(row.Name), "1", "L", fill, theme.SizeBody)
		fitCell(colWT[1], rowH, enc(groupLabel), "1", "C", fill, theme.SizeBody)
		pdf.Ln(-1)
	}

	// Layout constants (mm), A4 with 15 mm top/bottom margins → 267 mm usable.
	const (
		pageBottom  = 297.0 - 15.0 // Y of the bottom margin from the top of the page
		ovFirstH    = 16.0          // OV heading first page: CellFormat(h=12) + Ln(4)
		ovContH     = 14.0          // OV heading continuation: CellFormat(h=10) + Ln(4)
		tblHdrH     = 10.0          // table column-header row
		gapBetween  = 6.0           // vertical gap between Betreuende and Teilnehmende
	)

	evtH := 0.0
	if eventName != "" {
		evtH = 24.0 // title (14) + subtitle (6) + Ln(4)
	}

	// calcSectionPages pre-computes how many A4 pages one OV section occupies.
	// It mirrors the rendering logic exactly so that "Seite x von y" is accurate.
	calcSectionPages := func(nBet, nTN int) int {
		pages := 1
		y := 15.0 + evtH + ovFirstH

		// startTable: check whether header+first row fits; if not, new page.
		startTable := func() {
			if y+tblHdrH+rowH > pageBottom {
				pages++
				y = 15.0 + ovContH
			}
			y += tblHdrH
		}
		// nextRow: check whether the next row fits; if not, new page + repeat header.
		nextRow := func() {
			if y+rowH > pageBottom {
				pages++
				y = 15.0 + ovContH + tblHdrH
			}
			y += rowH
		}

		if nBet > 0 {
			startTable()
			for i := 0; i < nBet; i++ {
				nextRow()
			}
			y += gapBetween
		}
		if nTN > 0 {
			startTable()
			for i := 0; i < nTN; i++ {
				nextRow()
			}
		}
		return pages
	}

	for _, ovName := range ovNames {
		section := ovMap[ovName]
		totalPages := calcSectionPages(len(section.Betreuende), len(section.Teilnehmende))
		pageInSection := 1

		// doPageBreak starts a new page and renders the continuation OV heading.
		doPageBreak := func() {
			pageInSection++
			pdf.AddPage()
			theme.Font(pdf, "B", theme.SizeTitle)
			theme.TextColor(pdf, theme.ColorText)
			var heading string
			if totalPages > 1 {
				heading = fmt.Sprintf("Ortsverband: %s  (Seite %d von %d)", ovName, pageInSection, totalPages)
			} else {
				heading = "Ortsverband: " + ovName
			}
			pdf.CellFormat(0, 10, enc(heading), "", 1, "L", false, 0, "")
			pdf.Ln(4)
		}

		// startTable checks whether the table header + first row fit on the current
		// page; if not, it breaks to a new page first, then renders the header.
		startTable := func(renderHeader func()) {
			if pdf.GetY()+tblHdrH+rowH > pageBottom {
				doPageBreak()
			}
			renderHeader()
		}

		// nextRow checks whether the next data row fits; if not, breaks to a new
		// page and re-renders the table column header before the row.
		nextRow := func(renderHeader func()) {
			if pdf.GetY()+rowH > pageBottom {
				doPageBreak()
				renderHeader()
			}
		}

		pdf.AddPage()

		// ── Event header (first page of this OV section only) ──────────────
		if eventName != "" {
			theme.Font(pdf, "B", theme.SizeTitle+4)
			theme.TextColor(pdf, theme.ColorText)
			header := enc(eventName)
			if eventYear > 0 {
				header += fmt.Sprintf(" %d", eventYear)
			}
			pdf.CellFormat(0, 14, header, "", 1, "C", false, 0, "")
			theme.Font(pdf, "", theme.SizeSmall)
			theme.TextColor(pdf, theme.ColorSubtext)
			pdf.CellFormat(0, 6, enc("OV-Zuteilung"), "", 1, "C", false, 0, "")
			pdf.Ln(4)
		}

		// ── OV heading ─────────────────────────────────────────────────────
		theme.Font(pdf, "B", theme.SizeTitle)
		theme.TextColor(pdf, theme.ColorText)
		ovLine := "Ortsverband: " + ovName
		if totalPages > 1 {
			ovLine = fmt.Sprintf("Ortsverband: %s  (Seite 1 von %d)", ovName, totalPages)
		}
		pdf.CellFormat(0, 12, enc(ovLine), "", 1, "L", false, 0, "")
		pdf.Ln(4)

		// ── Betreuende table ───────────────────────────────────────────────
		if len(section.Betreuende) > 0 {
			startTable(renderBetreuendeHeader)
			for i, row := range section.Betreuende {
				nextRow(renderBetreuendeHeader)
				renderBetreuendeRow(row, i%2 == 0)
			}
			pdf.Ln(gapBetween)
		}

		// ── Teilnehmende table ─────────────────────────────────────────────
		if len(section.Teilnehmende) > 0 {
			startTable(renderTeilnehmendeHeader)
			for i, row := range section.Teilnehmende {
				nextRow(renderTeilnehmendeHeader)
				renderTeilnehmendeRow(row, i%2 == 0)
			}
		}
	}

	outputPath := filepath.Join(pdfOutputDir, "OV-Zuteilung.pdf")
	return pdf.OutputFileAndClose(outputPath)
}
