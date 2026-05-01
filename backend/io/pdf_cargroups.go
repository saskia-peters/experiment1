package io

import (
	"fmt"
	"path/filepath"

	"THW-JugendOlympiade/backend/config"
	"THW-JugendOlympiade/backend/models"

	"github.com/go-pdf/fpdf"
)

// GenerateCarGroupsPDF creates a PDF showing the CarGroup vehicle-pool
// assignments (one page per CarGroup). It is only generated when
// verteilungsmodus = "FixGroupSize" and cargroups = "ja".
//
// Layout (portrait A4):
//   - Event header + subtitle
//   - Per CarGroup: heading, Gruppen table, Fahrzeuge table with driver + seats
func GenerateCarGroupsPDF(carGroups []*models.CarGroup, eventName string, eventYear int, groupNames []string, cfg config.Config) error {
	if len(carGroups) == 0 {
		return nil
	}
	if err := ensurePDFDirectory(); err != nil {
		return err
	}

	theme := DefaultTheme
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	for _, cg := range carGroups {
		pdf.AddPage()
		pageW, _ := pdf.GetPageSize()
		contentW := pageW - 30 // 15 mm margins each side

		// ── Event header ──────────────────────────────────────────────────────────
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
			pdf.CellFormat(0, 6, enc("Fahrzeugpool-Einteilung"), "", 1, "C", false, 0, "")
			pdf.Ln(6)
		}

		// ── CarGroup heading ──────────────────────────────────────────────────────
		theme.Font(pdf, "B", theme.SizeTitle)
		theme.TextColor(pdf, theme.ColorPrimary)
		pdf.CellFormat(0, 12, enc(fmt.Sprintf("Fahrzeugpool %d", cg.ID)), "", 1, "L", false, 0, "")
		pdf.SetDrawColor(theme.ColorPrimary[0], theme.ColorPrimary[1], theme.ColorPrimary[2])
		pdf.SetLineWidth(0.5)
		x, y := pdf.GetXY()
		pdf.Line(x, y, x+contentW, y)
		pdf.Ln(4)
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetLineWidth(0.2)

		// ── Gruppen section ───────────────────────────────────────────────────────
		theme.Font(pdf, "B", theme.SizeHeading)
		theme.TextColor(pdf, theme.ColorText)
		pdf.CellFormat(0, 8, enc("Gruppen"), "", 1, "L", false, 0, "")
		pdf.Ln(1)

		// Table header
		colW := []float64{10, 60, 30, 35, 45}
		headers := []string{"#", "Gruppenname", "Teilnehmende", "Betreuende", enc("Personen gesamt")}
		theme.Font(pdf, "B", theme.SizeTableHeader)
		theme.FillColor(pdf, theme.ColorTableHeader)
		theme.TextColor(pdf, theme.ColorText)
		for i, h := range headers {
			pdf.CellFormat(colW[i], 8, enc(h), "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)

		// Table rows
		totalPeople := 0
		theme.Font(pdf, "", theme.SizeBody)
		for rowIdx, g := range cg.Groups {
			fill := rowIdx%2 == 1
			if fill {
				theme.FillColor(pdf, theme.ColorTableRowAlt)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			theme.TextColor(pdf, theme.ColorText)
			gName := config.GetGroupName(g.GroupID, groupNames)
			headcount := len(g.Teilnehmende) + len(g.Betreuende)
			totalPeople += headcount
			pdf.CellFormat(colW[0], 7, fmt.Sprintf("%d", g.GroupID), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(colW[1], 7, enc(gName), "1", 0, "L", fill, 0, "")
			pdf.CellFormat(colW[2], 7, fmt.Sprintf("%d", len(g.Teilnehmende)), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(colW[3], 7, fmt.Sprintf("%d", len(g.Betreuende)), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(colW[4], 7, fmt.Sprintf("%d", headcount), "1", 0, "C", fill, 0, "")
			pdf.Ln(-1)
		}
		// Total row
		theme.FillColor(pdf, theme.ColorTableHeader)
		theme.Font(pdf, "B", theme.SizeBody)
		pdf.CellFormat(colW[0]+colW[1]+colW[2]+colW[3], 7, enc("Gesamt"), "1", 0, "R", true, 0, "")
		pdf.CellFormat(colW[4], 7, fmt.Sprintf("%d", totalPeople), "1", 0, "C", true, 0, "")
		pdf.Ln(-1)
		pdf.Ln(6)

		// ── Fahrzeuge section ─────────────────────────────────────────────────────
		theme.Font(pdf, "B", theme.SizeHeading)
		theme.TextColor(pdf, theme.ColorText)
		pdf.CellFormat(0, 8, enc("Fahrzeuge"), "", 1, "L", false, 0, "")
		pdf.Ln(1)

		// Table header: Fahrzeug (OV) | Fahrer | Sitze
		carColW := []float64{100, 55, 25}
		carHeaders := []string{"Fahrzeug (OV)", "Fahrer", "Sitze"}
		theme.Font(pdf, "B", theme.SizeTableHeader)
		theme.FillColor(pdf, theme.ColorTableHeader)
		theme.TextColor(pdf, theme.ColorText)
		for i, h := range carHeaders {
			pdf.CellFormat(carColW[i], 8, enc(h), "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)

		// Table rows
		totalSeats := 0
		theme.Font(pdf, "", theme.SizeBody)
		for rowIdx, car := range cg.Cars {
			fill := rowIdx%2 == 1
			if fill {
				theme.FillColor(pdf, theme.ColorTableRowAlt)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			theme.TextColor(pdf, theme.ColorText)
			totalSeats += car.Sitzplaetze
			fahrzeugLabel := car.Bezeichnung
			if car.Ortsverband != "" {
				fahrzeugLabel += " (" + car.Ortsverband + ")"
			}
			fahrer := car.FahrerName
			if fahrer == "" {
				fahrer = "KEIN FAHRER bekannt!"
			}
			pdf.CellFormat(carColW[0], 7, enc(fahrzeugLabel), "1", 0, "L", fill, 0, "")
			pdf.CellFormat(carColW[1], 7, enc(fahrer), "1", 0, "L", fill, 0, "")
			pdf.CellFormat(carColW[2], 7, fmt.Sprintf("%d", car.Sitzplaetze), "1", 0, "C", fill, 0, "")
			pdf.Ln(-1)
		}
		// Total + free seats row
		frei := totalSeats - totalPeople
		freiStr := fmt.Sprintf("%d frei", frei)
		if frei < 0 {
			freiStr = fmt.Sprintf("%d zu wenig", -frei)
		}
		theme.FillColor(pdf, theme.ColorTableHeader)
		theme.Font(pdf, "B", theme.SizeBody)
		pdf.CellFormat(carColW[0]+carColW[1], 7, enc(fmt.Sprintf("Gesamt (%s)", freiStr)), "1", 0, "R", true, 0, "")
		pdf.CellFormat(carColW[2], 7, fmt.Sprintf("%d", totalSeats), "1", 0, "C", true, 0, "")
		pdf.Ln(-1)
	}

	outPath := filepath.Join(pdfOutputDir, "CarGroups.pdf")
	return pdf.OutputFileAndClose(outPath)
}
