package models

const (
	SheetName           = "Teilnehmende"
	BetreuendeSheetName = "Betreuende"
	StationsSheetName   = "Stationen"
	FahrzeugeSheetName  = "Fahrzeuge"
	TableName           = "teilnehmende"
)

// DbFile is the path to the SQLite database file.
// It is set at startup from config and defaults to "data.db".
var DbFile = "data.db"

// Teilnehmende represents a participant
type Teilnehmende struct {
	ID             int
	TeilnehmendeID int
	Name           string
	Ortsverband    string
	Alter          int
	Geschlecht     string
	PreGroup       string
}

// Betreuende represents a caretaker/driver for a group
type Betreuende struct {
	ID            int
	Name          string
	Ortsverband   string
	Fahrerlaubnis bool // true = "ja" in the Excel sheet
}

// Fahrzeug represents a vehicle used to transport a group
type Fahrzeug struct {
	ID          int
	Bezeichnung string // Name/description of the vehicle
	Ortsverband string
	Funkrufname string // Radio callsign
	FahrerName  string // Name of the driver (must be a Betreuende with Fahrerlaubnis)
	Sitzplaetze int    // Total seat count including the driver
}

// Group represents a group of participants
type Group struct {
	GroupID      int
	Teilnehmende []Teilnehmende
	Betreuende   []Betreuende
	Fahrzeuge    []Fahrzeug
	Ortsverbands map[string]int
	Geschlechts  map[string]int
	AlterSum     int
}

// GroupScore represents a group's score at a station
type GroupScore struct {
	GroupID int
	Score   int
}

// Station represents a station with groups that visited and their scores
type Station struct {
	StationID   int
	StationName string
	GroupScores []GroupScore
}

// GroupEvaluation represents a group's total score across all stations
type GroupEvaluation struct {
	GroupID      int
	TotalScore   int
	StationCount int
}

// OrtsverbandEvaluation represents an ortsverband's score statistics
type OrtsverbandEvaluation struct {
	Ortsverband      string
	TotalScore       int
	ParticipantCount int
	AverageScore     float64
}
