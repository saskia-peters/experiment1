package io

import (
	"database/sql"
	"fmt"
	"path/filepath"

	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/models"

	"github.com/go-pdf/fpdf"
)

// GeneratePDFReport creates a PDF report with one group per page.
// Pass the in-memory CarGroups slice when running in CarGroups mode so each
// group page can show its pool information instead of an empty vehicle section.
func GeneratePDFReport(db *sql.DB, eventName string, eventYear int, carGroups []*models.CarGroup) error {
	if err := ensurePDFDirectory(); err != nil {
		return err
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		return fmt.Errorf("failed to get groups: %w", err)
	}
	if len(groups) == 0 {
		return fmt.Errorf("no groups found to generate report")
	}

	// Build a lookup: GroupID → *CarGroup so the vehicle section can reference
	// the pool this group belongs to.
	type carGroupInfo struct {
		poolID int
		cars   []models.Fahrzeug
		peers  []int // GroupIDs of the other groups sharing this pool
	}
	cgByGroup := make(map[int]carGroupInfo)
	for _, cg := range carGroups {
		groupIDs := make([]int, 0, len(cg.Groups))
		for _, g := range cg.Groups {
			groupIDs = append(groupIDs, g.GroupID)
		}
		for _, g := range cg.Groups {
			peers := make([]int, 0, len(groupIDs)-1)
			for _, id := range groupIDs {
				if id != g.GroupID {
					peers = append(peers, id)
				}
			}
			cgByGroup[g.GroupID] = carGroupInfo{
				poolID: cg.ID,
				cars:   cg.Cars,
				peers:  peers,
			}
		}
	}

	theme := DefaultTheme
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)

	for _, group := range groups {
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
			pdf.CellFormat(0, 6, "Gruppeneinteilung", "", 1, "C", false, 0, "")
			pdf.Ln(4)
		}

		// Title
		theme.Font(pdf, "B", theme.SizeTitle)
		theme.TextColor(pdf, theme.ColorText)
		pdf.CellFormat(0, 15, fmt.Sprintf("Gruppe %d", group.GroupID), "", 1, "C", false, 0, "")
		pdf.Ln(5)

		// Participant count
		theme.Font(pdf, "", theme.SizeBody)
		theme.TextColor(pdf, theme.ColorSubtext)
		pdf.CellFormat(0, 8, fmt.Sprintf("Anzahl Teilnehmende: %d", len(group.Teilnehmende)), "", 1, "L", false, 0, "")
		pdf.Ln(3)

		// Table header
		theme.Font(pdf, "B", theme.SizeTableHeader)
		theme.FillColor(pdf, theme.ColorTableHeader)
		theme.TextColor(pdf, theme.ColorText)

		colWidths := []float64{60, 60, 50}
		for i, header := range []string{"Name", "Ortsverband", "Alter"} {
			pdf.CellFormat(colWidths[i], 10, header, "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)

		// Table rows
		theme.Font(pdf, "", theme.SizeBody)
		theme.FillColor(pdf, theme.ColorTableRowAlt)

		for i, t := range group.Teilnehmende {
			fill := i%2 == 0
			pdf.CellFormat(colWidths[0], 9, enc(t.Name), "1", 0, "L", fill, 0, "")
			pdf.CellFormat(colWidths[1], 9, enc(t.Ortsverband), "1", 0, "L", fill, 0, "")
			pdf.CellFormat(colWidths[2], 9, fmt.Sprintf("%d", t.Alter), "1", 0, "C", fill, 0, "")
			pdf.Ln(-1)
		}

		// Betreuende section
		if len(group.Betreuende) > 0 {
			pdf.Ln(6)
			theme.Font(pdf, "B", theme.SizeTableHeader)
			theme.TextColor(pdf, theme.ColorText)
			theme.FillColor(pdf, theme.ColorTableHeader)
			pdf.CellFormat(80, 10, "Betreuende", "1", 0, "C", true, 0, "")
			pdf.CellFormat(70, 10, "Ortsverband", "1", 0, "C", true, 0, "")
			pdf.CellFormat(30, 10, "Fahrerlaubnis", "1", 0, "C", true, 0, "")
			pdf.Ln(-1)

			theme.Font(pdf, "", theme.SizeBody)
			for i, b := range group.Betreuende {
				fill := i%2 == 0
				theme.FillColor(pdf, theme.ColorTableRowAlt)
				fahrerlaubnisStr := "nein"
				if b.Fahrerlaubnis {
					fahrerlaubnisStr = "ja"
				}
				if b.IsExternalDriver {
					fahrerlaubnisStr = "extern"
				}
				pdf.CellFormat(80, 9, enc(b.Name), "1", 0, "L", fill, 0, "")
				pdf.CellFormat(70, 9, enc(b.Ortsverband), "1", 0, "L", fill, 0, "")
				pdf.CellFormat(30, 9, fahrerlaubnisStr, "1", 0, "C", fill, 0, "")
				pdf.Ln(-1)
			}
		}

		// Fahrzeuge / Fahrzeugpool section
		if cgInfo, inPool := cgByGroup[group.GroupID]; inPool {
			// ── CarGroups mode: show pool heading + shared vehicles ──────────────
			pdf.Ln(6)
			theme.Font(pdf, "B", theme.SizeTableHeader)
			theme.TextColor(pdf, theme.ColorText)
			pdf.CellFormat(0, 9, enc(fmt.Sprintf("Fahrzeugpool %d", cgInfo.poolID)), "", 1, "L", false, 0, "")
			if len(cgInfo.peers) > 0 {
				peerStr := "Gemeinsam mit:"
				for _, pid := range cgInfo.peers {
					peerStr += fmt.Sprintf(" Gruppe %d", pid)
				}
				theme.Font(pdf, "", theme.SizeSmall)
				theme.TextColor(pdf, theme.ColorSubtext)
				pdf.CellFormat(0, 6, enc(peerStr), "", 1, "L", false, 0, "")
			}
			pdf.Ln(2)

			theme.Font(pdf, "B", theme.SizeTableHeader)
			theme.FillColor(pdf, theme.ColorTableHeader)
			theme.TextColor(pdf, theme.ColorText)
			pdf.CellFormat(100, 10, enc("Fahrzeug (OV)"), "1", 0, "C", true, 0, "")
			pdf.CellFormat(55, 10, enc("Fahrer"), "1", 0, "C", true, 0, "")
			pdf.CellFormat(25, 10, enc("Sitze"), "1", 0, "C", true, 0, "")
			pdf.Ln(-1)

			theme.Font(pdf, "", theme.SizeBody)
			totalSeats := 0
			for i, f := range cgInfo.cars {
				fill := i%2 == 0
				theme.FillColor(pdf, theme.ColorTableRowAlt)
				fahrzeugLabel := f.Bezeichnung
				if f.Ortsverband != "" {
					fahrzeugLabel += " (" + f.Ortsverband + ")"
				}
				fahrer := f.FahrerName
				if fahrer == "" {
					fahrer = "KEIN FAHRER bekannt!"
				}
				pdf.CellFormat(100, 9, enc(fahrzeugLabel), "1", 0, "L", fill, 0, "")
				pdf.CellFormat(55, 9, enc(fahrer), "1", 0, "L", fill, 0, "")
				pdf.CellFormat(25, 9, fmt.Sprintf("%d", f.Sitzplaetze), "1", 0, "C", fill, 0, "")
				pdf.Ln(-1)
				totalSeats += f.Sitzplaetze
			}

			// Seat summary counts all people in the pool, not just this group.
			poolPeople := 0
			for _, cg := range carGroups {
				if cg.ID == cgInfo.poolID {
					for _, g := range cg.Groups {
						poolPeople += len(g.Teilnehmende) + len(g.Betreuende)
					}
					break
				}
			}
			pdf.Ln(3)
			if poolPeople > totalSeats {
				theme.Font(pdf, "B", theme.SizeBody)
				pdf.SetTextColor(200, 0, 0)
				pdf.CellFormat(0, 8, enc(fmt.Sprintf(
					"Pool: %d Personen, nur %d Sitzpl\u00e4tze",
					poolPeople, totalSeats,
				)), "", 1, "L", false, 0, "")
				theme.TextColor(pdf, theme.ColorText)
			} else {
				theme.Font(pdf, "", theme.SizeSmall)
				theme.TextColor(pdf, theme.ColorSubtext)
				pdf.CellFormat(0, 6, enc(fmt.Sprintf(
					"Pool gesamt: %d Personen, %d Sitzpl\u00e4tze (%d frei)",
					poolPeople, totalSeats, totalSeats-poolPeople,
				)), "", 1, "L", false, 0, "")
			}
		} else if len(group.Fahrzeuge) > 0 {
			// ── Normal mode: direct vehicle assignment ────────────────────────────
			pdf.Ln(6)
			theme.Font(pdf, "B", theme.SizeTableHeader)
			theme.TextColor(pdf, theme.ColorText)
			theme.FillColor(pdf, theme.ColorTableHeader)
			pdf.CellFormat(50, 10, enc("Fahrzeug"), "1", 0, "C", true, 0, "")
			pdf.CellFormat(40, 10, enc("Funkrufname"), "1", 0, "C", true, 0, "")
			pdf.CellFormat(55, 10, enc("Fahrer"), "1", 0, "C", true, 0, "")
			pdf.CellFormat(35, 10, enc("Sitzpl\u00e4tze"), "1", 0, "C", true, 0, "")
			pdf.Ln(-1)

			theme.Font(pdf, "", theme.SizeBody)
			totalSeats := 0
			for i, f := range group.Fahrzeuge {
				fill := i%2 == 0
				theme.FillColor(pdf, theme.ColorTableRowAlt)
				pdf.CellFormat(50, 9, enc(f.Bezeichnung), "1", 0, "L", fill, 0, "")
				pdf.CellFormat(40, 9, enc(f.Funkrufname), "1", 0, "L", fill, 0, "")
				pdf.CellFormat(55, 9, enc(f.FahrerName), "1", 0, "L", fill, 0, "")
				pdf.CellFormat(35, 9, fmt.Sprintf("%d", f.Sitzplaetze), "1", 0, "C", fill, 0, "")
				pdf.Ln(-1)
				totalSeats += f.Sitzplaetze
			}

			totalPeople := len(group.Teilnehmende) + len(group.Betreuende)
			pdf.Ln(3)
			if totalPeople > totalSeats {
				theme.Font(pdf, "B", theme.SizeBody)
				pdf.SetTextColor(200, 0, 0)
				pdf.CellFormat(0, 8, enc(fmt.Sprintf(
					"Zu wenig Sitzpl\u00e4tze f\u00fcr Anzahl Gruppenmitglieder (%d Personen, %d Sitzpl\u00e4tze)",
					totalPeople, totalSeats,
				)), "", 1, "L", false, 0, "")
				theme.TextColor(pdf, theme.ColorText)
			} else {
				theme.Font(pdf, "", theme.SizeSmall)
				theme.TextColor(pdf, theme.ColorSubtext)
				pdf.CellFormat(0, 6, enc(fmt.Sprintf(
					"Gesamt: %d Personen, %d Sitzpl\u00e4tze",
					totalPeople, totalSeats,
				)), "", 1, "L", false, 0, "")
			}
		} else {
			pdf.Ln(6)
			theme.Font(pdf, "B", theme.SizeBody)
			pdf.SetTextColor(200, 0, 0)
			pdf.CellFormat(0, 8, enc("Kein Fahrzeug dieser Gruppe zugewiesen"), "", 1, "L", false, 0, "")
			theme.TextColor(pdf, theme.ColorText)
		}
	}

	if err = pdf.OutputFileAndClose(filepath.Join(pdfOutputDir, "Gruppeneinteilung.pdf")); err != nil {
		return fmt.Errorf("failed to save PDF: %w", err)
	}
	return nil
}
