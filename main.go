package main

import (
	"fmt"
	"log"
)

func main() {
	// Read XLSX file
	rows, err := readXLSXFile()
	if err != nil {
		log.Fatalf("Failed to read XLSX file: %v", err)
	}

	// Initialize SQLite database
	db, err := initDatabase()
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Insert data into database
	if err := insertData(db, rows); err != nil {
		log.Fatalf("Failed to insert data: %v", err)
	}

	fmt.Printf("Successfully imported %d rows from '%s' sheet into SQLite database\n", len(rows)-1, sheetName)

	// Create balanced groups
	if err := createBalancedGroups(db); err != nil {
		log.Fatalf("Failed to create groups: %v", err)
	}

	fmt.Println("Successfully created balanced groups")

	// Generate PDF report
	if err := generatePDFReport(db); err != nil {
		log.Fatalf("Failed to generate PDF report: %v", err)
	}

	fmt.Println("PDF report generated successfully: groups_report.pdf")
}
