package handlers

import (
	"context"
	"fmt"

	"THW-JugendOlympiade/backend/io"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ConvertMasterExcel opens a file dialog to select a source ("Master") Excel
// file, transforms it into the standard import format expected by LoadFile,
// then offers a save dialog for the converted output.
//
// event must be "Jugend" or "Mini" and controls which rows are selected.
func ConvertMasterExcel(ctx context.Context, event string) map[string]interface{} {
	ev := io.MasterEvent(event)
	if ev != io.EventJugend && ev != io.EventMini {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Unbekannte Veranstaltung %q – erlaubt sind \"Jugend\" oder \"Mini\"", event),
		}
	}
	// Step 1: choose source file
	srcPath, err := runtime.OpenFileDialog(ctx, runtime.OpenDialogOptions{
		Title: "Master-Excel auswählen",
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
			"message": fmt.Sprintf("Datei-Dialog konnte nicht geöffnet werden: %v", err),
		}
	}
	if srcPath == "" {
		return map[string]interface{}{
			"status":  "cancelled",
			"message": "Auswahl abgebrochen",
		}
	}

	// Step 2: read all sheets from the source file
	data, err := io.ReadMasterExcel(srcPath)
	if err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Quelldatei konnte nicht gelesen werden: %v", err),
		}
	}

	// Step 3: transform into the typed ConvertedData structure
	converted := io.TransformMasterExcel(data, ev)

	// Step 4: choose destination for the converted file
	destPath, err := runtime.SaveFileDialog(ctx, runtime.SaveDialogOptions{
		Title:           "Konvertierte Datei speichern",
		DefaultFilename: "konvertiert.xlsx",
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
			"message": fmt.Sprintf("Speicher-Dialog konnte nicht geöffnet werden: %v", err),
		}
	}
	if destPath == "" {
		return map[string]interface{}{
			"status":  "cancelled",
			"message": "Speichern abgebrochen",
		}
	}

	// Step 5: write the converted file
	if err := io.WriteMasterExcel(destPath, converted); err != nil {
		return map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Datei konnte nicht gespeichert werden: %v", err),
		}
	}

	return map[string]interface{}{
		"status":   "ok",
		"message":  fmt.Sprintf("Datei erfolgreich gespeichert: %s", destPath),
		"destPath": destPath,
	}
}
