package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"os"

	"experiment1/backend/database"
	"experiment1/backend/io"
	"experiment1/backend/models"
	"experiment1/backend/services"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend
var assets embed.FS

// App struct
type App struct {
	ctx context.Context
	db  *sql.DB
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// shutdown is called when the app is closing
func (a *App) shutdown(ctx context.Context) {
	if a.db != nil {
		a.db.Close()
	}
}

func main() {
	// Create an instance of the app structure
	app := NewApp()

	// Create application with options
	err := wails.Run(&options.App{
		Title:  "Jugendolympiade Verwaltung",
		Width:  1024,
		Height: 768,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		OnStartup:  app.startup,
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
	})

	if err != nil {
		println("Error:", err.Error())
	}
}

// CheckDB checks if the database has any data
func (a *App) CheckDB() map[string]interface{} {
	hasData := false
	count := 0

	if a.db != nil {
		var rowCount int
		err := a.db.QueryRow("SELECT COUNT(*) FROM teilnehmer").Scan(&rowCount)
		if err == nil && rowCount > 0 {
			hasData = true
			count = rowCount
		}
	}

	return map[string]interface{}{
		"hasData": hasData,
		"count":   count,
	}
}

// LoadFile opens a file dialog and loads the selected Excel file
func (a *App) LoadFile() map[string]interface{} {
	// Open file dialog
	filePath, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Excel File",
		Filters: []runtime.FileFilter{
			{
				DisplayName: "Excel Files (*.xlsx)",
				Pattern:     "*.xlsx",
			},
		},
	})

	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to open file dialog: %v", err),
		}
	}

	if filePath == "" {
		// User cancelled
		return map[string]interface{}{
			"status":  "cancelled",
			"message": "File selection cancelled",
		}
	}

	// Read XLSX file
	rows, err := io.ReadXLSXFile(filePath)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to read XLSX file: %v", err),
		}
	}

	// Close previous database if any
	if a.db != nil {
		a.db.Close()
		a.db = nil
	}

	// Remove old database file to start fresh
	os.Remove(models.DbFile)

	// Initialize SQLite database
	db, err := database.InitDatabase()
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to initialize database: %v", err),
		}
	}
	a.db = db

	// Insert data into database
	if err := database.InsertData(a.db, rows); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to insert data: %v", err),
		}
	}

	// Read and insert stations from Stationen sheet
	stationRows, err := io.ReadStationsFromXLSX(filePath)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to read stations: %v", err),
		}
	}

	if err := database.InsertStations(a.db, stationRows); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to insert stations: %v", err),
		}
	}

	// Create balanced groups
	if err := services.CreateBalancedGroups(a.db); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to create groups: %v", err),
		}
	}

	participantCount := len(rows) - 1

	return map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Successfully loaded %d participants and created balanced groups", participantCount),
		"count":   participantCount,
	}
}

// ShowGroups retrieves and returns groups from the database
func (a *App) ShowGroups() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Please load an Excel file first",
		}
	}

	// Retrieve groups from database
	groups, err := database.GetGroupsForReport(a.db)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to retrieve groups: %v", err),
		}
	}

	if len(groups) == 0 {
		return map[string]interface{}{
			"status":  "error",
			"message": "No groups found. Please load a file first.",
		}
	}

	return map[string]interface{}{
		"status": "success",
		"count":  len(groups),
		"groups": groups,
	}
}

// ShowStations retrieves and returns stations with group scores from the database
func (a *App) ShowStations() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Please load an Excel file first",
		}
	}

	// Retrieve stations from database
	stations, err := database.GetStationsForReport(a.db)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to retrieve stations: %v", err),
		}
	}

	if len(stations) == 0 {
		return map[string]interface{}{
			"status":  "error",
			"message": "No stations found. Please ensure your Excel file has a 'Stationen' sheet.",
		}
	}

	return map[string]interface{}{
		"status":   "success",
		"count":    len(stations),
		"stations": stations,
	}
}

// GetAllGroups retrieves all group IDs from the database
func (a *App) GetAllGroups() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Please load an Excel file first",
		}
	}

	groupIDs, err := database.GetAllGroupIDs(a.db)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to retrieve groups: %v", err),
		}
	}

	return map[string]interface{}{
		"status": "success",
		"groups": groupIDs,
	}
}

// AssignScore assigns a score to a group at a station
func (a *App) AssignScore(groupID int, stationID int, score int) map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Please load an Excel file first",
		}
	}

	err := database.AssignGroupStationScore(a.db, groupID, stationID, score)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to assign score: %v", err),
		}
	}

	return map[string]interface{}{
		"status":  "success",
		"message": fmt.Sprintf("Score %d assigned to Group %d", score, groupID),
	}
}

// GetGroupEvaluations retrieves all groups with their total scores ranked from high to low
func (a *App) GetGroupEvaluations() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Please load an Excel file first",
		}
	}

	evaluations, err := database.GetGroupEvaluations(a.db)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to retrieve group evaluations: %v", err),
		}
	}

	if len(evaluations) == 0 {
		return map[string]interface{}{
			"status":  "error",
			"message": "No groups found with scores.",
		}
	}

	return map[string]interface{}{
		"status":      "success",
		"evaluations": evaluations,
	}
}

// GetOrtsverbandEvaluations retrieves all ortsverbands with their total scores ranked from high to low
func (a *App) GetOrtsverbandEvaluations() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Please load an Excel file first",
		}
	}

	evaluations, err := database.GetOrtsverbandEvaluations(a.db)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to retrieve ortsverband evaluations: %v", err),
		}
	}

	if len(evaluations) == 0 {
		return map[string]interface{}{
			"status":  "error",
			"message": "No ortsverbands found with scores.",
		}
	}

	return map[string]interface{}{
		"status":      "success",
		"evaluations": evaluations,
	}
}

// GeneratePDF generates a PDF report
func (a *App) GeneratePDF() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Please load an Excel file first",
		}
	}

	// Generate PDF report
	if err := io.GeneratePDFReport(a.db); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to generate PDF report: %v", err),
		}
	}

	absPath, _ := os.Getwd()

	return map[string]interface{}{
		"status":  "success",
		"message": "PDF report generated successfully",
		"file":    "groups_report.pdf",
		"path":    absPath + string(os.PathSeparator) + "pdfdocs" + string(os.PathSeparator) + "groups_report.pdf",
	}
}

// GenerateGroupEvaluationPDF generates a PDF report for group rankings
func (a *App) GenerateGroupEvaluationPDF() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Please load an Excel file first",
		}
	}

	// Generate PDF report
	if err := io.GenerateGroupEvaluationPDF(a.db); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to generate PDF report: %v", err),
		}
	}

	absPath, _ := os.Getwd()

	return map[string]interface{}{
		"status":  "success",
		"message": "Group evaluation PDF generated successfully",
		"file":    "group_evaluations.pdf",
		"path":    absPath + string(os.PathSeparator) + "pdfdocs" + string(os.PathSeparator) + "group_evaluations.pdf",
	}
}

// GenerateOrtsverbandEvaluationPDF generates a PDF report for ortsverband rankings
func (a *App) GenerateOrtsverbandEvaluationPDF() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Please load an Excel file first",
		}
	}

	// Generate PDF report
	if err := io.GenerateOrtsverbandEvaluationPDF(a.db); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to generate PDF report: %v", err),
		}
	}

	absPath, _ := os.Getwd()

	return map[string]interface{}{
		"status":  "success",
		"message": "Ortsverband evaluation PDF generated successfully",
		"file":    "ortsverband_evaluations.pdf",
		"path":    absPath + string(os.PathSeparator) + "pdfdocs" + string(os.PathSeparator) + "ortsverband_evaluations.pdf",
	}
}

// GenerateParticipantCertificates generates participant certificates PDF
func (a *App) GenerateParticipantCertificates() map[string]interface{} {
	if a.db == nil {
		return map[string]interface{}{
			"status":  "error",
			"message": "Please load an Excel file first",
		}
	}

	// Generate PDF report
	if err := io.GenerateParticipantCertificates(a.db); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to generate PDF certificates: %v", err),
		}
	}

	absPath, _ := os.Getwd()

	return map[string]interface{}{
		"status":  "success",
		"message": "Participant certificates PDF generated successfully",
		"file":    "participant_certificates.pdf",
		"path":    absPath + string(os.PathSeparator) + "pdfdocs" + string(os.PathSeparator) + "participant_certificates.pdf",
	}
}
