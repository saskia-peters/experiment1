package io

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"sort"

	"THW-JugendOlympiade/backend/config"
	"THW-JugendOlympiade/backend/database"

	"github.com/go-pdf/fpdf"
)

// tnCard holds the data needed to print one participant card.
type tnCard struct {
	Name        string
	Ortsverband string
	GroupID     int
}

// GenerateTeilnehmendeCardsPDF creates a PDF of A6-sized participant cards
// on A4 landscape pages (2×2 grid per page). Each card shows the participant's
// name, Ortsverband and group. The pages are intended to be cut into 4 after
// printing so each participant receives one card.
func GenerateTeilnehmendeCardsPDF(db *sql.DB, eventName string, eventYear int, groupNames []string) error {
	if err := ensurePDFDirectory(); err != nil {
		return err
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		return fmt.Errorf("failed to get groups: %w", err)
	}
	if len(groups) == 0 {
		return fmt.Errorf("no groups found to generate participant cards")
	}

	// Build flat list of participant cards sorted by Ortsverband then name.
	var cards []tnCard
	for _, group := range groups {
		for _, t := range group.Teilnehmende {
			cards = append(cards, tnCard{
				Name:        t.Name,
				Ortsverband: t.Ortsverband,
				GroupID:     group.GroupID,
			})
		}
	}
	sort.Slice(cards, func(i, j int) bool {
		if cards[i].Ortsverband != cards[j].Ortsverband {
			return cards[i].Ortsverband < cards[j].Ortsverband
		}
		return cards[i].Name < cards[j].Name
	})

	// A4 landscape: 297 × 210 mm.  Cards fill the full page (no margins).
	// 2 columns × 2 rows → each card is 148.5 × 105 mm (= A6 landscape).
	const (
		cardW   = 148.5
		cardH   = 105.0
		padding = 8.0 // inner spacing around card content
	)

	// Card origins (top-left corner), column-major order left-to-right, top-to-bottom.
	origins := [4][2]float64{
		{0, 0},
		{cardW, 0},
		{0, cardH},
		{cardW, cardH},
	}

	pdf := fpdf.New("L", "mm", "A4", "")
	pdf.SetMargins(0, 0, 0)
	pdf.SetAutoPageBreak(false, 0)

	for i, card := range cards {
		if i%4 == 0 {
			pdf.AddPage()
		}

		cx := origins[i%4][0]
		cy := origins[i%4][1]

		// Frame — draw a 1 mm inset so cut lines remain visible.
		pdf.SetDrawColor(0, 0, 0)
		pdf.SetLineWidth(0.3)
		pdf.Rect(cx+0.5, cy+0.5, cardW-1, cardH-1, "D")

		// Optionally print a small event label in the top-left corner.
		if eventName != "" {
			pdf.SetFont("Arial", "", 7)
			pdf.SetTextColor(160, 160, 160)
			label := enc(eventName)
			if eventYear > 0 {
				label += fmt.Sprintf(" %d", eventYear)
			}
			pdf.SetXY(cx+padding, cy+padding)
			pdf.CellFormat(cardW-2*padding, 5, label, "", 0, "L", false, 0, "")
		}

		// Vertical centre of the card content block:
		//   Name (12mm) + gap 3 + OV (8mm) + gap 6 + Gruppe (12mm) = 41mm
		//   centre-start = cy + (cardH-41)/2
		contentStartY := cy + (cardH-41)/2

		// Name — large, bold, centred.
		pdf.SetFont("Arial", "B", 20)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(cx+padding, contentStartY)
		pdf.CellFormat(cardW-2*padding, 12, enc(card.Name), "", 0, "C", false, 0, "")

		// Ortsverband — closer to name (gap 3 instead of 5), smaller, grey, centred.
		pdf.SetFont("Arial", "", 13)
		pdf.SetTextColor(80, 80, 80)
		pdf.SetXY(cx+padding, contentStartY+15)
		pdf.CellFormat(cardW-2*padding, 8, enc(card.Ortsverband), "", 0, "C", false, 0, "")

		// Gruppe — "Gruppe N - Name", larger font, centred.
		groupLabel := fmt.Sprintf("Gruppe %d - %s", card.GroupID, config.GetGroupName(card.GroupID, groupNames))
		pdf.SetFont("Arial", "B", 16)
		pdf.SetTextColor(0, 0, 0)
		pdf.SetXY(cx+padding, contentStartY+29)
		pdf.CellFormat(cardW-2*padding, 12, enc(groupLabel), "", 0, "C", false, 0, "")
	}

	outputPath := filepath.Join(pdfOutputDir, "Teilnehmende-Karten.pdf")
	return pdf.OutputFileAndClose(outputPath)
}
