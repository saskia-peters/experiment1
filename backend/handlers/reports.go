package handlers

import (
	"database/sql"
	"fmt"
	"os"

	"THW-JugendOlympiade/backend/io"
)

// GeneratePDF generates the groups PDF report and the station recording sheets PDF.
func GeneratePDF(db *sql.DB, eventName string, eventYear int, groupNames []string) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Bitte zuerst eine Excel-Datei laden.",
		}
	}
	if err := io.GeneratePDFReport(db, eventName, eventYear); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Gruppen-PDF konnte nicht erstellt werden: %v", err),
		}
	}
	if err := io.GenerateStationSheetsPDF(db, eventName, eventYear, groupNames); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Stationslaufzettel-PDF konnte nicht erstellt werden: %v", err),
		}
	}
	absPath, _ := os.Getwd()
	sep := string(os.PathSeparator)
	return map[string]interface{}{
		"status":  "success",
		"message": "Gruppen-PDF und Stationslaufzettel erfolgreich erstellt",
		"file":    "Gruppeneinteilung.pdf",
		"path":    absPath + sep + "pdfdocs" + sep + "Gruppeneinteilung.pdf",
		"file2":   "Stationslaufzettel.pdf",
		"path2":   absPath + sep + "pdfdocs" + sep + "Stationslaufzettel.pdf",
	}
}

// GenerateGroupEvaluationPDF generates a PDF report for group rankings.
func GenerateGroupEvaluationPDF(db *sql.DB) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Bitte zuerst eine Excel-Datei laden.",
		}
	}
	if err := io.GenerateGroupEvaluationPDF(db); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Auswertungs-PDF konnte nicht erstellt werden: %v", err),
		}
	}
	absPath, _ := os.Getwd()
	return map[string]interface{}{
		"status":  "success",
		"message": "Auswertungs-PDF erfolgreich erstellt",
		"file":    "Auswertung_nach_Gruppe.pdf",
		"path":    absPath + string(os.PathSeparator) + "pdfdocs" + string(os.PathSeparator) + "Auswertung_nach_Gruppe.pdf",
	}
}

// GenerateOrtsverbandEvaluationPDF generates a PDF report for ortsverband rankings.
func GenerateOrtsverbandEvaluationPDF(db *sql.DB) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Bitte zuerst eine Excel-Datei laden.",
		}
	}
	if err := io.GenerateOrtsverbandEvaluationPDF(db); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Ortsverband-Auswertungs-PDF konnte nicht erstellt werden: %v", err),
		}
	}
	absPath, _ := os.Getwd()
	return map[string]interface{}{
		"status":  "success",
		"message": "Ortsverband-Auswertungs-PDF erfolgreich erstellt",
		"file":    "Auswertung_nach_Ortsverband.pdf",
		"path":    absPath + string(os.PathSeparator) + "pdfdocs" + string(os.PathSeparator) + "Auswertung_nach_Ortsverband.pdf",
	}
}
