package io

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/models"

	"github.com/go-pdf/fpdf"
)

// GenerateOverviewPDF creates a single-page (or multi-page) summary PDF.
//
// Sections:
//  1. Totals – Teilnehmende, Betreuende mit/ohne Fahrerlaubnis
//  2. Per-OV breakdown – TN count and Betreuende count per Ortsverband
//  3. Integrity checks – persons assigned more than once, duplicate drivers
//  4. Carpool capacity summary (only when carGroups is non-empty)
func GenerateOverviewPDF(db *sql.DB, eventName string, eventYear int, carGroups []*models.CarGroup) error {
	if err := ensurePDFDirectory(); err != nil {
		return err
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		return fmt.Errorf("failed to get groups: %w", err)
	}
	if len(groups) == 0 {
		return fmt.Errorf("no groups found to generate overview")
	}

	// ── Build aggregate data ──────────────────────────────────────────────────

	type ovRow struct {
		OV          string
		TN          int
		BetMitFaL   int // Betreuende with Fahrerlaubnis
		BetOhneFaL  int // Betreuende without Fahrerlaubnis
		BetTotal    int
	}

	ovMap := make(map[string]*ovRow)

	totalTN := 0
	totalBetMit := 0
	totalBetOhne := 0

	// Integrity tracking
	tnSeen := make(map[int][]int)   // TeilnehmendeID → group IDs
	betSeen := make(map[int][]int)  // Betreuende.ID → group IDs
	driverSeen := make(map[string][]int) // lower-cased driver name → group IDs

	for _, g := range groups {
		for _, t := range g.Teilnehmende {
			totalTN++
			ov := t.Ortsverband
			if _, ok := ovMap[ov]; !ok {
				ovMap[ov] = &ovRow{OV: ov}
			}
			ovMap[ov].TN++
			tnSeen[t.TeilnehmendeID] = append(tnSeen[t.TeilnehmendeID], g.GroupID)
		}
		for _, b := range g.Betreuende {
			ov := b.Ortsverband
			if _, ok := ovMap[ov]; !ok {
				ovMap[ov] = &ovRow{OV: ov}
			}
			ovMap[ov].BetTotal++
			if b.Fahrerlaubnis {
				totalBetMit++
				ovMap[ov].BetMitFaL++
			} else {
				totalBetOhne++
				ovMap[ov].BetOhneFaL++
			}
			betSeen[b.ID] = append(betSeen[b.ID], g.GroupID)
		}
		for _, f := range g.Fahrzeuge {
			if f.FahrerName != "" {
				key := strings.ToLower(strings.TrimSpace(f.FahrerName))
				driverSeen[key] = append(driverSeen[key], g.GroupID)
			}
		}
	}

	// Sort OV names
	ovNames := make([]string, 0, len(ovMap))
	for ov := range ovMap {
		ovNames = append(ovNames, ov)
	}
	sort.Strings(ovNames)

	// Collect integrity issues
	type dupEntry struct {
		Label    string
		GroupIDs []int
	}
	var dupTN, dupBet, dupDriver []dupEntry

	for tid, gids := range tnSeen {
		if len(gids) > 1 {
			// Find name from groups
			name := fmt.Sprintf("ID %d", tid)
			for _, g := range groups {
				for _, t := range g.Teilnehmende {
					if t.TeilnehmendeID == tid {
						name = t.Name
						break
					}
				}
			}
			dupTN = append(dupTN, dupEntry{Label: name, GroupIDs: gids})
		}
	}
	sort.Slice(dupTN, func(i, j int) bool { return dupTN[i].Label < dupTN[j].Label })

	for bid, gids := range betSeen {
		if len(gids) > 1 {
			name := fmt.Sprintf("ID %d", bid)
			for _, g := range groups {
				for _, b := range g.Betreuende {
					if b.ID == bid {
						name = b.Name
						break
					}
				}
			}
			dupBet = append(dupBet, dupEntry{Label: name, GroupIDs: gids})
		}
	}
	sort.Slice(dupBet, func(i, j int) bool { return dupBet[i].Label < dupBet[j].Label })

	for dname, gids := range driverSeen {
		if len(gids) > 1 {
			dupDriver = append(dupDriver, dupEntry{Label: dname, GroupIDs: gids})
		}
	}
	sort.Slice(dupDriver, func(i, j int) bool { return dupDriver[i].Label < dupDriver[j].Label })

	// ── Carpool data ──────────────────────────────────────────────────────────
	type poolRow struct {
		ID        int
		TotalSeats int
		UsedSeats  int
	}
	var poolRows []poolRow
	grandSeats := 0
	grandUsed := 0
	for _, cg := range carGroups {
		seats := 0
		for _, c := range cg.Cars {
			seats += c.Sitzplaetze
		}
		used := 0
		for _, g := range cg.Groups {
			used += len(g.Teilnehmende) + len(g.Betreuende)
		}
		poolRows = append(poolRows, poolRow{ID: cg.ID, TotalSeats: seats, UsedSeats: used})
		grandSeats += seats
		grandUsed += used
	}

	// ── Render PDF ────────────────────────────────────────────────────────────
	theme := DefaultTheme
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(15, 15, 15)
	pdf.SetAutoPageBreak(true, 15)
	pdf.AddPage()

	// Helper: section heading with a coloured rule underneath
	sectionHeading := func(title string) {
		pdf.Ln(4)
		theme.Font(pdf, "B", theme.SizeHeading)
		theme.TextColor(pdf, theme.ColorPrimary)
		pdf.CellFormat(0, 8, enc(title), "", 1, "L", false, 0, "")
		pdf.SetDrawColor(theme.ColorPrimary[0], theme.ColorPrimary[1], theme.ColorPrimary[2])
		pdf.SetLineWidth(0.4)
		x, y := pdf.GetXY()
		pageW, _ := pdf.GetPageSize()
		pdf.Line(x, y, pageW-15, y)
		pdf.Ln(3)
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetLineWidth(0.2)
		theme.TextColor(pdf, theme.ColorText)
	}

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
		pdf.CellFormat(0, 6, enc("Übersicht"), "", 1, "C", false, 0, "")
		pdf.Ln(4)
	}

	// ── Section 1: Totals ─────────────────────────────────────────────────────
	sectionHeading("Gesamtübersicht")

	colW2 := []float64{120, 60}
	renderKV := func(label, value string, fill bool) {
		if fill {
			theme.FillColor(pdf, theme.ColorTableRowAlt)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		theme.Font(pdf, "", theme.SizeBody)
		theme.TextColor(pdf, theme.ColorText)
		pdf.CellFormat(colW2[0], 8, enc(label), "1", 0, "L", fill, 0, "")
		pdf.CellFormat(colW2[1], 8, enc(value), "1", 0, "C", fill, 0, "")
		pdf.Ln(-1)
	}
	renderKV("Anzahl Gruppen", fmt.Sprintf("%d", len(groups)), false)
	renderKV("Teilnehmende gesamt", fmt.Sprintf("%d", totalTN), true)
	renderKV("Betreuende gesamt", fmt.Sprintf("%d", totalBetMit+totalBetOhne), false)
	renderKV("  davon mit Fahrerlaubnis", fmt.Sprintf("%d", totalBetMit), true)
	renderKV("  davon ohne Fahrerlaubnis", fmt.Sprintf("%d", totalBetOhne), false)
	renderKV("Personen gesamt (TN + Betreuende)", fmt.Sprintf("%d", totalTN+totalBetMit+totalBetOhne), true)

	// ── Section 2: Per-OV breakdown ───────────────────────────────────────────
	sectionHeading("Aufschlüsselung nach Ortsverband")

	ovColW := []float64{64, 29, 29, 29, 29}
	// Table header
	theme.Font(pdf, "B", theme.SizeTableHeader)
	theme.FillColor(pdf, theme.ColorTableHeader)
	theme.TextColor(pdf, theme.ColorText)
	for _, hdr := range []struct {
		text string
		w    float64
	}{
		{"Ortsverband", ovColW[0]},
		{"TN", ovColW[1]},
		{"Bet. m.Fal.", ovColW[2]},
		{"Bet. o.Fal.", ovColW[3]},
		{"Bet. gesamt", ovColW[4]},
	} {
		pdf.CellFormat(hdr.w, 8, enc(hdr.text), "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	for idx, ov := range ovNames {
		r := ovMap[ov]
		fill := idx%2 == 0
		if fill {
			theme.FillColor(pdf, theme.ColorTableRowAlt)
		} else {
			pdf.SetFillColor(255, 255, 255)
		}
		theme.Font(pdf, "", theme.SizeBody)
		theme.TextColor(pdf, theme.ColorText)
		pdf.CellFormat(ovColW[0], 7, enc(ov), "1", 0, "L", fill, 0, "")
		pdf.CellFormat(ovColW[1], 7, fmt.Sprintf("%d", r.TN), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(ovColW[2], 7, fmt.Sprintf("%d", r.BetMitFaL), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(ovColW[3], 7, fmt.Sprintf("%d", r.BetOhneFaL), "1", 0, "C", fill, 0, "")
		pdf.CellFormat(ovColW[4], 7, fmt.Sprintf("%d", r.BetTotal), "1", 0, "C", fill, 0, "")
		pdf.Ln(-1)
	}
	// Total row
	theme.FillColor(pdf, theme.ColorTableHeader)
	theme.Font(pdf, "B", theme.SizeBody)
	pdf.CellFormat(ovColW[0], 7, enc("Gesamt"), "1", 0, "L", true, 0, "")
	pdf.CellFormat(ovColW[1], 7, fmt.Sprintf("%d", totalTN), "1", 0, "C", true, 0, "")
	pdf.CellFormat(ovColW[2], 7, fmt.Sprintf("%d", totalBetMit), "1", 0, "C", true, 0, "")
	pdf.CellFormat(ovColW[3], 7, fmt.Sprintf("%d", totalBetOhne), "1", 0, "C", true, 0, "")
	pdf.CellFormat(ovColW[4], 7, fmt.Sprintf("%d", totalBetMit+totalBetOhne), "1", 0, "C", true, 0, "")
	pdf.Ln(-1)

	// ── Page 2: Integrity checks + Carpool capacity ─────────────────────────
	pdf.AddPage()
	if eventName != "" {
		theme.Font(pdf, "B", theme.SizeTitle+4)
		theme.TextColor(pdf, theme.ColorText)
		header2 := enc(eventName)
		if eventYear > 0 {
			header2 += fmt.Sprintf(" %d", eventYear)
		}
		pdf.CellFormat(0, 14, header2, "", 1, "C", false, 0, "")
		theme.Font(pdf, "", theme.SizeSmall)
		theme.TextColor(pdf, theme.ColorSubtext)
		pdf.CellFormat(0, 6, enc("Übersicht"), "", 1, "C", false, 0, "")
		pdf.Ln(4)
	}

	// ── Section 3: Integrity checks ───────────────────────────────────────────
	sectionHeading("Integritätsprüfung")

	renderCheck := func(label string, issues []dupEntry) {
		theme.Font(pdf, "B", theme.SizeBody)
		theme.TextColor(pdf, theme.ColorText)
		if len(issues) == 0 {
			pdf.SetTextColor(0, 140, 0) // green
			pdf.CellFormat(0, 7, enc("[OK]  "+label+": keine Dopplungen"), "", 1, "L", false, 0, "")
			theme.TextColor(pdf, theme.ColorText)
			return
		}
		pdf.SetTextColor(200, 0, 0) // red
		pdf.CellFormat(0, 7, enc(fmt.Sprintf("[!!]  %s: %d Dopplung(en)", label, len(issues))), "", 1, "L", false, 0, "")
		theme.TextColor(pdf, theme.ColorText)
		theme.Font(pdf, "", theme.SizeSmall)
		for _, d := range issues {
			gidStrs := make([]string, len(d.GroupIDs))
			for i, id := range d.GroupIDs {
				gidStrs[i] = fmt.Sprintf("Gruppe %d", id)
			}
			pdf.CellFormat(0, 6, enc(fmt.Sprintf("    %s  ->  %s", d.Label, strings.Join(gidStrs, ", "))), "", 1, "L", false, 0, "")
		}
	}

	renderCheck("Teilnehmende", dupTN)
	renderCheck("Betreuende", dupBet)
	renderCheck("Fahrer", dupDriver)

	// ── Section 4: Carpool capacity (only when pools exist) ───────────────────
	if len(poolRows) > 0 {
		sectionHeading("Fahrzeugpool-Kapazität")

		poolColW := []float64{30, 50, 50, 50}
		theme.Font(pdf, "B", theme.SizeTableHeader)
		theme.FillColor(pdf, theme.ColorTableHeader)
		theme.TextColor(pdf, theme.ColorText)
		for _, hdr := range []struct {
			text string
			w    float64
		}{
			{"Pool", poolColW[0]},
			{"Sitzplätze gesamt", poolColW[1]},
			{"Personen (belegt)", poolColW[2]},
			{"Freie Plätze", poolColW[3]},
		} {
			pdf.CellFormat(hdr.w, 8, enc(hdr.text), "1", 0, "C", true, 0, "")
		}
		pdf.Ln(-1)

		for idx, pr := range poolRows {
			fill := idx%2 == 0
			if fill {
				theme.FillColor(pdf, theme.ColorTableRowAlt)
			} else {
				pdf.SetFillColor(255, 255, 255)
			}
			free := pr.TotalSeats - pr.UsedSeats
			theme.Font(pdf, "", theme.SizeBody)
			theme.TextColor(pdf, theme.ColorText)
			pdf.CellFormat(poolColW[0], 7, fmt.Sprintf("%d", pr.ID), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(poolColW[1], 7, fmt.Sprintf("%d", pr.TotalSeats), "1", 0, "C", fill, 0, "")
			pdf.CellFormat(poolColW[2], 7, fmt.Sprintf("%d", pr.UsedSeats), "1", 0, "C", fill, 0, "")
			// Highlight rows where people exceed seats
			if free < 0 {
				pdf.SetTextColor(200, 0, 0)
			}
			pdf.CellFormat(poolColW[3], 7, fmt.Sprintf("%d", free), "1", 0, "C", fill, 0, "")
			theme.TextColor(pdf, theme.ColorText)
			pdf.Ln(-1)
		}
		// Total row
		grandFree := grandSeats - grandUsed
		theme.FillColor(pdf, theme.ColorTableHeader)
		theme.Font(pdf, "B", theme.SizeBody)
		pdf.CellFormat(poolColW[0], 7, enc("Ges."), "1", 0, "C", true, 0, "")
		pdf.CellFormat(poolColW[1], 7, fmt.Sprintf("%d", grandSeats), "1", 0, "C", true, 0, "")
		pdf.CellFormat(poolColW[2], 7, fmt.Sprintf("%d", grandUsed), "1", 0, "C", true, 0, "")
		if grandFree < 0 {
			pdf.SetTextColor(200, 0, 0)
		}
		pdf.CellFormat(poolColW[3], 7, fmt.Sprintf("%d", grandFree), "1", 0, "C", true, 0, "")
		theme.TextColor(pdf, theme.ColorText)
		pdf.Ln(-1)

		// Overall fit indicator
		pdf.Ln(2)
		theme.Font(pdf, "B", theme.SizeBody)
		if grandFree >= 0 {
			pdf.SetTextColor(0, 140, 0)
			pdf.CellFormat(0, 7, enc(fmt.Sprintf("[OK]  Alle %d Personen passen in die Fahrzeuge (%d freie Plaetze)", grandUsed, grandFree)), "", 1, "L", false, 0, "")
		} else {
			pdf.SetTextColor(200, 0, 0)
			pdf.CellFormat(0, 7, enc(fmt.Sprintf("[!!]  %d Personen passen NICHT in die Fahrzeuge (%d Plaetze fehlen)", grandUsed, -grandFree)), "", 1, "L", false, 0, "")
		}
		theme.TextColor(pdf, theme.ColorText)
	}

	outputPath := filepath.Join(pdfOutputDir, "Uebersicht.pdf")
	return pdf.OutputFileAndClose(outputPath)
}
