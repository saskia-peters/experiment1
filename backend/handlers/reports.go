package handlers

import (
	"database/sql"
	"fmt"
	"os"
	"strings"

	"THW-JugendOlympiade/backend/config"
	"THW-JugendOlympiade/backend/io"
	"THW-JugendOlympiade/backend/services"
)

// GeneratePDF generates the groups PDF report and the station recording sheets PDF.
func GeneratePDF(db *sql.DB, eventName string, eventYear int, groupNames []string, cfg config.Config) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Bitte zuerst eine Excel-Datei laden.",
		}
	}
	if err := io.GeneratePDFReport(db, eventName, eventYear, services.GetLastCarGroups(), groupNames); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Gruppen-PDF konnte nicht erstellt werden: %v", err),
		}
	}
	if err := io.GenerateStationSheetsPDF(db, eventName, eventYear, groupNames); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Stationsbewertungszettel-PDF konnte nicht erstellt werden: %v", err),
		}
	}
	if err := io.GenerateOVAssignmentsPDF(db, eventName, eventYear, groupNames); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("OV-Zuteilungs-PDF konnte nicht erstellt werden: %v", err),
		}
	}
	if err := io.GenerateTeilnehmendeCardsPDF(db, eventName, eventYear, groupNames); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Teilnehmende-Karten-PDF konnte nicht erstellt werden: %v", err),
		}
	}
	if err := io.GenerateOverviewPDF(db, eventName, eventYear, services.GetLastCarGroups()); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Übersichts-PDF konnte nicht erstellt werden: %v", err),
		}
	}

	// CarGroups PDF — only when FixGroupSize mode with cargroups = "ja".
	absPath, _ := os.Getwd()
	sep := string(os.PathSeparator)
	result := map[string]interface{}{
		"status":  "success",
		"message": "Gruppen-PDF, Stationsbewertungszettel, OV-Zuteilung, Teilnehmende-Karten und Übersicht erfolgreich erstellt",
		"file":    "Gruppeneinteilung.pdf",
		"path":    absPath + sep + "pdfdocs" + sep + "Gruppeneinteilung.pdf",
		"file2":   "Stationsbewertungszettel.pdf",
		"path2":   absPath + sep + "pdfdocs" + sep + "Stationsbewertungszettel.pdf",
		"file3":   "OV-Zuteilung.pdf",
		"path3":   absPath + sep + "pdfdocs" + sep + "OV-Zuteilung.pdf",
		"file4":   "Teilnehmende-Karten.pdf",
		"path4":   absPath + sep + "pdfdocs" + sep + "Teilnehmende-Karten.pdf",
		"file5":   "Uebersicht.pdf",
		"path5":   absPath + sep + "pdfdocs" + sep + "Uebersicht.pdf",
	}
	if cfg.Verteilung.Verteilungsmodus == "FixGroupSize" && strings.EqualFold(cfg.Verteilung.CarGroups, "ja") {
		carGroups := services.GetLastCarGroups()
		if len(carGroups) > 0 {
			if err := io.GenerateCarGroupsPDF(carGroups, eventName, eventYear, groupNames, cfg); err != nil {
				return map[string]interface{}{
					"status":  "error",
					"message": fmt.Sprintf("CarGroups-PDF konnte nicht erstellt werden: %v", err),
				}
			}
			result["message"] = "Gruppen-PDF, Stationsbewertungszettel, OV-Zuteilung, Teilnehmende-Karten, Übersicht und CarGroups-PDF erfolgreich erstellt"
			result["file6"] = "CarGroups.pdf"
			result["path6"] = absPath + sep + "pdfdocs" + sep + "CarGroups.pdf"
		}
	}
	return result
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
