package database

import (
	"database/sql"
	"fmt"

	"THW-JugendOlympiade/backend/models"
)

// GetAllTeilnehmende reads all participants from the database
func GetAllTeilnehmende(db *sql.DB) ([]models.Teilnehmende, error) {
	rows, err := db.Query("SELECT id, teilnehmer_id, name, ortsverband, age, geschlecht, pregroup FROM teilnehmende ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teilnehmende []models.Teilnehmende
	for rows.Next() {
		var t models.Teilnehmende
		var alter sql.NullInt64
		err := rows.Scan(&t.ID, &t.TeilnehmendeID, &t.Name, &t.Ortsverband, &alter, &t.Geschlecht, &t.PreGroup)
		if err != nil {
			return nil, err
		}
		if alter.Valid {
			t.Alter = int(alter.Int64)
		}
		teilnehmende = append(teilnehmende, t)
	}

	return teilnehmende, rows.Err()
}

// GetGroupsForReport retrieves all groups with their participants from the database
func GetGroupsForReport(db *sql.DB) ([]models.Group, error) {
	// Single query to get all groups and participants with JOIN
	query := `
		SELECT r.group_id, t.id, t.teilnehmer_id, t.name, t.ortsverband, t.age, t.geschlecht, t.pregroup
		FROM gruppe r
		INNER JOIN teilnehmende t ON t.teilnehmer_id = r.teilnehmer_id
		ORDER BY r.group_id, t.name
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map to build groups efficiently
	groupMap := make(map[int]*models.Group)
	var groupOrder []int

	for rows.Next() {
		var groupID int
		var t models.Teilnehmende
		var alter sql.NullInt64

		err := rows.Scan(&groupID, &t.ID, &t.TeilnehmendeID, &t.Name, &t.Ortsverband, &alter, &t.Geschlecht, &t.PreGroup)
		if err != nil {
			return nil, err
		}

		if alter.Valid {
			t.Alter = int(alter.Int64)
		}

		// Get or create group
		group, exists := groupMap[groupID]
		if !exists {
			group = &models.Group{
				GroupID:      groupID,
				Teilnehmende: make([]models.Teilnehmende, 0),
				Ortsverbands: make(map[string]int),
				Geschlechts:  make(map[string]int),
			}
			groupMap[groupID] = group
			groupOrder = append(groupOrder, groupID)
		}

		// Add participant to group
		group.Teilnehmende = append(group.Teilnehmende, t)

		// Update group statistics
		group.Ortsverbands[t.Ortsverband]++
		group.Geschlechts[t.Geschlecht]++
		group.AlterSum += t.Alter
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Load betreuende for each group
	bRows, err := db.Query(`
		SELECT gb.group_id, b.id, b.name, b.ortsverband, b.fahrerlaubnis
		FROM gruppe_betreuende gb
		INNER JOIN betreuende b ON b.id = gb.betreuende_id
		ORDER BY gb.group_id, b.name
	`)
	if err != nil {
		return nil, err
	}
	defer bRows.Close()
	for bRows.Next() {
		var groupID int
		var b models.Betreuende
		var fahrerlaubnis int
		if err := bRows.Scan(&groupID, &b.ID, &b.Name, &b.Ortsverband, &fahrerlaubnis); err != nil {
			return nil, err
		}
		b.Fahrerlaubnis = fahrerlaubnis != 0
		if g, ok := groupMap[groupID]; ok {
			g.Betreuende = append(g.Betreuende, b)
		}
	}
	if err := bRows.Err(); err != nil {
		return nil, err
	}

	// Load fahrzeuge for each group
	fRows, err := db.Query(`
		SELECT gf.group_id, f.id, f.bezeichnung, f.ortsverband, f.funkrufname, f.fahrer_name, f.sitzplaetze
		FROM gruppe_fahrzeuge gf
		INNER JOIN fahrzeuge f ON f.id = gf.fahrzeug_id
		ORDER BY gf.group_id, f.bezeichnung
	`)
	if err != nil {
		return nil, err
	}
	defer fRows.Close()
	for fRows.Next() {
		var groupID int
		var f models.Fahrzeug
		if err := fRows.Scan(&groupID, &f.ID, &f.Bezeichnung, &f.Ortsverband, &f.Funkrufname, &f.FahrerName, &f.Sitzplaetze); err != nil {
			return nil, err
		}
		if g, ok := groupMap[groupID]; ok {
			g.Fahrzeuge = append(g.Fahrzeuge, f)
		}
	}
	if err := fRows.Err(); err != nil {
		return nil, err
	}

	// Convert map to slice in correct order
	groups := make([]models.Group, 0, len(groupMap))
	for _, groupID := range groupOrder {
		groups = append(groups, *groupMap[groupID])
	}

	return groups, nil
}

// GetAllBetreuende returns all caretakers from the database
func GetAllBetreuende(db *sql.DB) ([]models.Betreuende, error) {
	rows, err := db.Query("SELECT id, name, ortsverband, fahrerlaubnis FROM betreuende ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Betreuende
	for rows.Next() {
		var b models.Betreuende
		var fahrerlaubnis int
		if err := rows.Scan(&b.ID, &b.Name, &b.Ortsverband, &fahrerlaubnis); err != nil {
			return nil, err
		}
		b.Fahrerlaubnis = fahrerlaubnis != 0
		result = append(result, b)
	}
	return result, rows.Err()
}

// GetAllFahrzeuge returns all vehicles from the database
func GetAllFahrzeuge(db *sql.DB) ([]models.Fahrzeug, error) {
	rows, err := db.Query("SELECT id, bezeichnung, ortsverband, funkrufname, fahrer_name, sitzplaetze FROM fahrzeuge ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []models.Fahrzeug
	for rows.Next() {
		var f models.Fahrzeug
		if err := rows.Scan(&f.ID, &f.Bezeichnung, &f.Ortsverband, &f.Funkrufname, &f.FahrerName, &f.Sitzplaetze); err != nil {
			return nil, err
		}
		result = append(result, f)
	}
	return result, rows.Err()
}

// GetStationsForReport retrieves all stations with group scores from the database
func GetStationsForReport(db *sql.DB) ([]models.Station, error) {
	// Single query to get all stations with their scores using LEFT JOIN
	// (LEFT JOIN ensures we get stations even if they have no scores yet)
	query := `
		SELECT 
			s.station_id, 
			s.station_name,
			gss.group_id,
			gss.score
		FROM stations s
		LEFT JOIN group_station_scores gss ON s.station_id = gss.station_id
		ORDER BY s.station_name, gss.score DESC, gss.group_id ASC
	`

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Map to build stations efficiently
	stationMap := make(map[int]*models.Station)
	var stationOrder []int

	for rows.Next() {
		var stationID int
		var stationName string
		var groupID sql.NullInt64
		var score sql.NullInt64

		err := rows.Scan(&stationID, &stationName, &groupID, &score)
		if err != nil {
			return nil, err
		}

		// Get or create station
		station, exists := stationMap[stationID]
		if !exists {
			station = &models.Station{
				StationID:   stationID,
				StationName: stationName,
				GroupScores: make([]models.GroupScore, 0),
			}
			stationMap[stationID] = station
			stationOrder = append(stationOrder, stationID)
		}

		// Add score if exists (groupID will be NULL if no scores for this station)
		if groupID.Valid {
			gs := models.GroupScore{
				GroupID: int(groupID.Int64),
			}
			if score.Valid {
				gs.Score = int(score.Int64)
			}
			station.GroupScores = append(station.GroupScores, gs)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Convert map to slice in correct order
	stations := make([]models.Station, 0, len(stationMap))
	for _, stationID := range stationOrder {
		stations = append(stations, *stationMap[stationID])
	}

	return stations, nil
}

// GetAllGroupIDs retrieves all group IDs from the database
func GetAllGroupIDs(db *sql.DB) ([]int, error) {
	rows, err := db.Query("SELECT DISTINCT group_id FROM gruppe ORDER BY group_id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groupIDs []int
	for rows.Next() {
		var groupID int
		if err := rows.Scan(&groupID); err != nil {
			return nil, err
		}
		groupIDs = append(groupIDs, groupID)
	}

	return groupIDs, rows.Err()
}

// GetStationNamesOrdered retrieves all station names ordered alphabetically.
// GroupScores is left empty — use GetStationsForReport when scores are needed.
func GetStationNamesOrdered(db *sql.DB) ([]models.Station, error) {
	rows, err := db.Query("SELECT station_id, station_name FROM stations ORDER BY station_name")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stations []models.Station
	for rows.Next() {
		var s models.Station
		if err := rows.Scan(&s.StationID, &s.StationName); err != nil {
			return nil, err
		}
		stations = append(stations, s)
	}
	return stations, rows.Err()
}

// GetDistinctOrtsverbands returns a sorted list of all Ortsverbands present
// across both teilnehmende and betreuende tables.
func GetDistinctOrtsverbands(db *sql.DB) ([]string, error) {
	query := `
		SELECT DISTINCT ortsverband FROM teilnehmende WHERE ortsverband != ''
		UNION
		SELECT DISTINCT ortsverband FROM betreuende  WHERE ortsverband != ''
		ORDER BY ortsverband`
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []string
	for rows.Next() {
		var ov string
		if err := rows.Scan(&ov); err != nil {
			return nil, err
		}
		result = append(result, ov)
	}
	return result, rows.Err()
}

// PersonRecord is a lightweight row used by the name-editor.
type PersonRecord struct {
	ID          int    `json:"id"`
	Kind        string `json:"kind"` // "teilnehmende" | "betreuende"
	Name        string `json:"name"`
	Ortsverband string `json:"ortsverband"`
}

// GetPersonenByOrtsverband returns all Teilnehmende and Betreuende for the
// given Ortsverband, sorted by kind then name.
func GetPersonenByOrtsverband(db *sql.DB, ortsverband string) ([]PersonRecord, error) {
	query := `
		SELECT id, 'teilnehmende' AS kind, name, ortsverband
		FROM teilnehmende
		WHERE ortsverband = ?
		UNION ALL
		SELECT id, 'betreuende' AS kind, name, ortsverband
		FROM betreuende
		WHERE ortsverband = ?
		ORDER BY kind, name`
	rows, err := db.Query(query, ortsverband, ortsverband)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PersonRecord
	for rows.Next() {
		var p PersonRecord
		if err := rows.Scan(&p.ID, &p.Kind, &p.Name, &p.Ortsverband); err != nil {
			return nil, err
		}
		result = append(result, p)
	}
	return result, rows.Err()
}

// UpdatePersonName updates the name field for a single row identified by id
// and kind ("teilnehmende" or "betreuende").
// Returns an error for unknown kinds to prevent SQL injection via table name.
func UpdatePersonName(db *sql.DB, kind string, id int, newName string) error {
	var table string
	switch kind {
	case "teilnehmende":
		table = "teilnehmende"
	case "betreuende":
		table = "betreuende"
	default:
		return fmt.Errorf("unbekannte Personenart %q", kind)
	}
	_, err := db.Exec("UPDATE "+table+" SET name = ? WHERE id = ?", newName, id)
	return err
}
