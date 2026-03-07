package main

const (
	dbFile       = "data.db"
	xlsxFile     = "data.xlsx"
	sheetName    = "Teilnehmer"
	tableName    = "teilnehmer"
	maxGroupSize = 8
)

// Teilnehmer represents a participant
type Teilnehmer struct {
	ID           int
	TeilnehmerID int
	Name         string
	Ortsverband  string
	Alter        int
	Geschlecht   string
}

// Group represents a group of participants
type Group struct {
	GroupID      int
	Teilnehmers  []Teilnehmer
	Ortsverbands map[string]int
	Geschlechts  map[string]int
	AlterSum     int
}
