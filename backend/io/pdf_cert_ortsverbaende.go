package io

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"THW-JugendOlympiade/backend/database"

	"github.com/jung-kurt/gofpdf"
)

// A4 page geometry constants (mm) used throughout ortsverband certificates.
const (
	ovPageW    = 210.0
	ovMarginLR = 15.0
	ovContentW = ovPageW - 2*ovMarginLR // 180 mm
)

// GenerateOrtsverbandCertificates creates one PDF page per Ortsverband.
//
// Ranking (from GetOrtsverbandEvaluations, ordered by average score desc):
//   - Index 0 → Siegerurkunde: shows ov_winner_image.png above "Bester Ortsverband"
//   - All others → Urkunde: identical layout, no ranking mentioned
//
// If cert_background_ov.png exists in the working directory it is rendered as a
// full-page background on every certificate page before the text content.
func GenerateOrtsverbandCertificates(db *sql.DB, eventYear int, eventName string) error {
	if err := ensurePDFDirectory(); err != nil {
		return err
	}

	evaluations, err := database.GetOrtsverbandEvaluations(db)
	if err != nil {
		return fmt.Errorf("failed to get ortsverband evaluations: %w", err)
	}
	if len(evaluations) == 0 {
		return fmt.Errorf("keine Ortsverbände mit Bewertungen gefunden")
	}

	// Build ortsverband → participant names map
	teilnehmende, err := database.GetAllTeilnehmende(db)
	if err != nil {
		return fmt.Errorf("failed to get participants: %w", err)
	}
	ovParticipants := make(map[string][]string)
	for _, t := range teilnehmende {
		if t.Ortsverband != "" {
			ovParticipants[t.Ortsverband] = append(ovParticipants[t.Ortsverband], t.Name)
		}
	}

	theme := DefaultTheme
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(ovMarginLR, ovMarginLR, ovMarginLR)
	pdf.SetAutoPageBreak(false, 0) // absolute positioning throughout

	const bgFile = "cert_background_ov.png"
	_, statErr := os.Stat(bgFile)
	useBg := statErr == nil

	currentYear := eventYear
	if currentYear == 0 {
		currentYear = time.Now().Year()
	}

	for i, eval := range evaluations {
		pdf.AddPage()
		if useBg {
			pdf.Image(bgFile, 0, 0, 210, 297, false, imageTypeFromFile(bgFile), 0, "")
		}
		if i == 0 {
			ovRenderWinner(pdf, theme, eval.Ortsverband, ovParticipants[eval.Ortsverband], currentYear, eventName)
		} else {
			ovRenderParticipant(pdf, theme, eval.Ortsverband, ovParticipants[eval.Ortsverband], currentYear, eventName)
		}
	}

	if err := pdf.OutputFileAndClose(filepath.Join(pdfOutputDir, "Urkunden_Ortsverbaende.pdf")); err != nil {
		return fmt.Errorf("failed to save PDF: %w", err)
	}
	return nil
}

// ovRenderWinner renders the Siegerurkunde for the best Ortsverband.
// ov_winner_image.png (140 mm wide, centred; height auto from aspect ratio)
// is placed above the "Bester Ortsverband" text.
//
// Vertical layout (y in mm):
//
//	 25  event name title
//	 44  year
//	 62  "Siegerurkunde"
//	 78  ortsverband name
//	 88  ov_winner_image.png  (140 mm wide, centred)
//	187  "Bester Ortsverband" (gold)
//	201  "Teilnehmende" section label
//	212  participant list
func ovRenderWinner(pdf *gofpdf.Fpdf, theme PDFTheme, ortsverband string, participants []string, year int, eventName string) {
	const left = ovMarginLR
	const w = ovContentW

	pdf.SetXY(left, 25)
	theme.Font(pdf, "B", theme.SizeCertTitle)
	theme.TextColor(pdf, theme.ColorPrimary)
	pdf.CellFormat(w, 14, enc(eventName), "", 0, "C", false, 0, "")

	pdf.SetXY(left, 44)
	theme.Font(pdf, "B", theme.SizeCertYear)
	theme.TextColor(pdf, theme.ColorPrimary)
	pdf.CellFormat(w, 12, fmt.Sprintf("%d", year), "", 0, "C", false, 0, "")

	pdf.SetXY(left, 62)
	theme.Font(pdf, "B", theme.SizeCertGroup)
	theme.TextColor(pdf, theme.ColorText)
	pdf.CellFormat(w, 12, "Siegerurkunde", "", 0, "C", false, 0, "")

	pdf.SetXY(left, 78)
	theme.Font(pdf, "B", theme.SizeCertName)
	theme.TextColor(pdf, theme.ColorText)
	pdf.CellFormat(w, 14, enc(ortsverband), "", 0, "C", false, 0, "")

	// Winner image: centred horizontally, 140 mm wide, height from aspect ratio.
	const imgW = 140.0
	pdf.Image("ov_winner_image.png", (ovPageW-imgW)/2, 88, imgW, 0, false, "", 0, "")

	pdf.SetXY(left, 187)
	theme.Font(pdf, "B", theme.SizeCertRank)
	theme.TextColor(pdf, theme.ColorAccent)
	pdf.CellFormat(w, 14, "Bester Ortsverband", "", 0, "C", false, 0, "")

	pdf.SetXY(left, 201)
	theme.Font(pdf, "B", theme.SizeCertGroup)
	theme.TextColor(pdf, theme.ColorText)
	pdf.CellFormat(w, 10, "Teilnehmende", "", 0, "C", false, 0, "")

	ovParticipantsList(pdf, theme, participants, left, w, 212)
}

// ovRenderParticipant renders the standard participation Urkunde.
// No image and no ranking are shown.
//
// Vertical layout (y in mm):
//
//	 40  event name title
//	 60  year
//	 80  "Urkunde"
//	100  ortsverband name
//	125  "Teilnehmende" section label
//	138  participant list
func ovRenderParticipant(pdf *gofpdf.Fpdf, theme PDFTheme, ortsverband string, participants []string, year int, eventName string) {
	const left = ovMarginLR
	const w = ovContentW

	pdf.SetXY(left, 40)
	theme.Font(pdf, "B", theme.SizeCertTitle)
	theme.TextColor(pdf, theme.ColorPrimary)
	pdf.CellFormat(w, 14, enc(eventName), "", 0, "C", false, 0, "")

	pdf.SetXY(left, 60)
	theme.Font(pdf, "B", theme.SizeCertYear)
	theme.TextColor(pdf, theme.ColorPrimary)
	pdf.CellFormat(w, 12, fmt.Sprintf("%d", year), "", 0, "C", false, 0, "")

	pdf.SetXY(left, 80)
	theme.Font(pdf, "B", theme.SizeCertGroup)
	theme.TextColor(pdf, theme.ColorText)
	pdf.CellFormat(w, 12, "Urkunde", "", 0, "C", false, 0, "")

	pdf.SetXY(left, 100)
	theme.Font(pdf, "B", theme.SizeCertName)
	theme.TextColor(pdf, theme.ColorText)
	pdf.CellFormat(w, 14, enc(ortsverband), "", 0, "C", false, 0, "")

	pdf.SetXY(left, 125)
	theme.Font(pdf, "B", theme.SizeCertGroup)
	theme.TextColor(pdf, theme.ColorText)
	pdf.CellFormat(w, 10, "Teilnehmende", "", 0, "C", false, 0, "")

	ovParticipantsList(pdf, theme, participants, left, w, 138)
}

// ovParticipantsList renders a plain list of participant names (no table borders
// or alternating fills), one name per line, centred, starting at startY (mm).
func ovParticipantsList(pdf *gofpdf.Fpdf, theme PDFTheme, names []string, left, width, startY float64) {
	pdf.SetXY(left, startY)
	theme.Font(pdf, "", theme.SizeCertOrtsverband)
	theme.TextColor(pdf, theme.ColorText)
	for _, name := range names {
		pdf.SetX(left)
		pdf.CellFormat(width, 6, enc(name), "", 1, "C", false, 0, "")
	}
}
