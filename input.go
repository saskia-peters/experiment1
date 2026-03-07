package main

import (
	"fmt"
	"log"
	"os"

	"github.com/xuri/excelize/v2"
)

// readXLSXFile reads the XLSX file and returns the rows
func readXLSXFile() ([][]string, error) {
	// Check if XLSX file exists
	if _, err := os.Stat(xlsxFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("XLSX file '%s' not found", xlsxFile)
	}

	// Open XLSX file
	f, err := excelize.OpenFile(xlsxFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open XLSX file: %w", err)
	}
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Failed to close XLSX file: %v", err)
		}
	}()

	// Read all rows from the "Teilnehmer" sheet
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("failed to read sheet '%s': %w", sheetName, err)
	}

	if len(rows) == 0 {
		return nil, fmt.Errorf("sheet '%s' is empty", sheetName)
	}

	return rows, nil
}
