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

	theme := DefaultTheme
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	// Column widths for Betreuende: Name 50 + Gruppe 50 + Fahrzeug 62 + Fhr. 18 = 180mm
	colWB := []float64{50, 50, 62, 18}
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
		headers := []string{"Betreuende", "Gruppe", "Fahrzeug", "Fhr."}
		for i, h := range headers {
			pdf.CellFormat(colWB[i], 10, enc(h), "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)
	}

	renderBetreuendeRow := func(row ovPersonRow, fill bool) {
		theme.Font(pdf, "", theme.SizeBody)
		theme.FillColor(pdf, theme.ColorTableRowAlt)
		groupLabel := fmt.Sprintf("Gruppe %d - %s", row.GroupID, config.GetGroupName(row.GroupID, groupNames))
		fitCell(colWB[0], rowH, enc(row.Name), "1", "L", fill, theme.SizeBody)
		fitCell(colWB[1], rowH, enc(groupLabel), "1", "C", fill, theme.SizeBody)
		fitCell(colWB[2], rowH, enc(row.Fahrzeug), "1", "L", fill, theme.SizeBody)
		check := ""
		if row.IsFahrer {
			check = "X"
		}
		pdf.CellFormat(colWB[3], rowH, check, "1", 0, "C", fill, 0, "")
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

	for _, ovName := range ovNames {
		section := ovMap[ovName]
		pdf.AddPage()

		// Event header
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
			pdf.CellFormat(0, 6, "OV-Zuteilung", "", 1, "C", false, 0, "")
			pdf.Ln(4)
		}

		// OV heading
		theme.Font(pdf, "B", theme.SizeTitle)
		theme.TextColor(pdf, theme.ColorText)
		pdf.CellFormat(0, 12, enc("Ortsverband: "+ovName), "", 1, "L", false, 0, "")
		pdf.Ln(4)

		// Betreuende table
		if len(section.Betreuende) > 0 {
			renderBetreuendeHeader()
			for i, row := range section.Betreuende {
				renderBetreuendeRow(row, i%2 == 0)
			}
			pdf.Ln(6)
		}

		// Teilnehmende table
		if len(section.Teilnehmende) > 0 {
			renderTeilnehmendeHeader()
			for i, row := range section.Teilnehmende {
				renderTeilnehmendeRow(row, i%2 == 0)
			}
		}
	}

	outputPath := filepath.Join(pdfOutputDir, "OV-Zuteilung.pdf")
	return pdf.OutputFileAndClose(outputPath)
}
