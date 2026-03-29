package database_test

import (
	"testing"

	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/models"
)

// ---------------------------------------------------------------------------
// GetAllTeilnehmende
// ---------------------------------------------------------------------------

func TestGetAllTeilnehmende_EmptyDB_ReturnsNil(t *testing.T) {
	db := newTestDB(t)
	result, err := database.GetAllTeilnehmende(db)
	if err != nil {
		t.Fatalf("GetAllTeilnehmende: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 participants, got %d", len(result))
	}
}

func TestGetAllTeilnehmende_ReturnsAllParticipants(t *testing.T) {
	db := newTestDB(t)
	mustInsertParticipants(t, db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", ""},
		{"Bob", "Hamburg", "30", "M", "Team1"},
	})

	result, err := database.GetAllTeilnehmende(db)
	if err != nil {
		t.Fatalf("GetAllTeilnehmende: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(result))
	}
	// first result is Alice (row index 1, lowest autoincrement id)
	if result[0].Name != "Alice" {
		t.Errorf("expected first participant Alice, got %q", result[0].Name)
	}
	if result[0].Ortsverband != "Berlin" {
		t.Errorf("expected Ortsverband Berlin, got %q", result[0].Ortsverband)
	}
	if result[0].Alter != 25 {
		t.Errorf("expected Alter 25, got %d", result[0].Alter)
	}
	if result[0].Geschlecht != "W" {
		t.Errorf("expected Geschlecht W, got %q", result[0].Geschlecht)
	}
}

func TestGetAllTeilnehmende_NullAge_ReturnsZero(t *testing.T) {
	db := newTestDB(t)
	// Insert directly with NULL age (InsertData stores empty string as "",
	// but production DB can have genuine NULLs stored via other paths).
	if _, err := db.Exec(
		"INSERT INTO teilnehmende (teilnehmer_id, name, ortsverband, age, geschlecht, pregroup) VALUES (1, 'Alice', 'Berlin', NULL, 'W', '')",
	); err != nil {
		t.Fatalf("direct insert: %v", err)
	}

	result, err := database.GetAllTeilnehmende(db)
	if err != nil {
		t.Fatalf("GetAllTeilnehmende: %v", err)
	}
	if len(result) == 0 {
		t.Fatal("expected 1 participant")
	}
	if result[0].Alter != 0 {
		t.Errorf("expected Alter=0 for NULL age, got %d", result[0].Alter)
	}
}

func TestGetAllTeilnehmende_OrderedByInsertionID(t *testing.T) {
	db := newTestDB(t)
	mustInsertParticipants(t, db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Charlie", "Berlin", "20", "M", ""},
		{"Alice", "Berlin", "22", "W", ""},
		{"Bob", "Berlin", "25", "M", ""},
	})

	result, err := database.GetAllTeilnehmende(db)
	if err != nil {
		t.Fatalf("GetAllTeilnehmende: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 participants, got %d", len(result))
	}
	// Ordered by autoincrement id (insertion order)
	want := []string{"Charlie", "Alice", "Bob"}
	for i, w := range want {
		if result[i].Name != w {
			t.Errorf("position %d: want %q, got %q", i, w, result[i].Name)
		}
	}
}

func TestGetAllTeilnehmende_PreGroupPreserved(t *testing.T) {
	db := newTestDB(t)
	mustInsertParticipants(t, db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", "TeamA"},
	})

	result, err := database.GetAllTeilnehmende(db)
	if err != nil {
		t.Fatalf("GetAllTeilnehmende: %v", err)
	}
	if result[0].PreGroup != "TeamA" {
		t.Errorf("expected PreGroup=TeamA, got %q", result[0].PreGroup)
	}
}

// ---------------------------------------------------------------------------
// GetAllBetreuende
// ---------------------------------------------------------------------------

func TestGetAllBetreuende_EmptyDB_ReturnsNil(t *testing.T) {
	db := newTestDB(t)
	result, err := database.GetAllBetreuende(db)
	if err != nil {
		t.Fatalf("GetAllBetreuende: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 betreuende, got %d", len(result))
	}
}

func TestGetAllBetreuende_ReturnsFahrerlaubnisAsBool(t *testing.T) {
	db := newTestDB(t)
	mustInsertBetreuende(t, db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Licensed", "Berlin", "ja"},
		{"Unlicensed", "Hamburg", "nein"},
	})

	result, err := database.GetAllBetreuende(db)
	if err != nil {
		t.Fatalf("GetAllBetreuende: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 betreuende, got %d", len(result))
	}

	// Find by name to avoid order dependency
	byName := make(map[string]models.Betreuende)
	for _, b := range result {
		byName[b.Name] = b
	}

	if !byName["Licensed"].Fahrerlaubnis {
		t.Errorf("'Licensed' should have Fahrerlaubnis=true")
	}
	if byName["Unlicensed"].Fahrerlaubnis {
		t.Errorf("'Unlicensed' should have Fahrerlaubnis=false")
	}
}

// ---------------------------------------------------------------------------
// SaveGroups + GetGroupsForReport (round-trip)
// ---------------------------------------------------------------------------

func TestGetGroupsForReport_EmptyDB_ReturnsEmpty(t *testing.T) {
	db := newTestDB(t)
	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 0 {
		t.Errorf("expected 0 groups, got %d", len(groups))
	}
}

func TestGetGroupsForReport_ReturnsGroupsWithParticipants(t *testing.T) {
	db := newTestDB(t)
	mustInsertParticipants(t, db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", ""},
		{"Bob", "Hamburg", "22", "M", ""},
		{"Carol", "Berlin", "28", "W", ""},
	})

	// Put Alice and Carol in group 1, Bob in group 2
	all, _ := database.GetAllTeilnehmende(db)
	groupsIn := []models.Group{
		{GroupID: 1, Teilnehmende: []models.Teilnehmende{all[0], all[2]}},
		{GroupID: 2, Teilnehmende: []models.Teilnehmende{all[1]}},
	}
	if err := database.SaveGroups(db, groupsIn); err != nil {
		t.Fatalf("SaveGroups: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Build id→count map
	sizes := make(map[int]int)
	for _, g := range groups {
		sizes[g.GroupID] = len(g.Teilnehmende)
	}
	if sizes[1] != 2 {
		t.Errorf("group 1: expected 2 members, got %d", sizes[1])
	}
	if sizes[2] != 1 {
		t.Errorf("group 2: expected 1 member, got %d", sizes[2])
	}
}

func TestGetGroupsForReport_ComputesOrtsverbandStatistics(t *testing.T) {
	db := newTestDB(t)
	mustInsertParticipants(t, db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", ""},
		{"Bob", "Berlin", "22", "M", ""},
		{"Carol", "Hamburg", "28", "W", ""},
	})

	all, _ := database.GetAllTeilnehmende(db)
	if err := database.SaveGroups(db, []models.Group{
		{GroupID: 1, Teilnehmende: all},
	}); err != nil {
		t.Fatalf("SaveGroups: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}

	g := groups[0]
	if g.Ortsverbands["Berlin"] != 2 {
		t.Errorf("expected Berlin count=2, got %d", g.Ortsverbands["Berlin"])
	}
	if g.Ortsverbands["Hamburg"] != 1 {
		t.Errorf("expected Hamburg count=1, got %d", g.Ortsverbands["Hamburg"])
	}
	if g.Geschlechts["W"] != 2 {
		t.Errorf("expected W count=2, got %d", g.Geschlechts["W"])
	}
}

func TestGetGroupsForReport_IncludesBetreuende(t *testing.T) {
	db := newTestDB(t)
	mustInsertParticipants(t, db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", ""},
	})
	mustInsertBetreuende(t, db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Trainer", "Berlin", "ja"},
	})

	all, _ := database.GetAllTeilnehmende(db)
	if err := database.SaveGroups(db, []models.Group{
		{GroupID: 1, Teilnehmende: all},
	}); err != nil {
		t.Fatalf("SaveGroups: %v", err)
	}

	var betreuerID int
	db.QueryRow("SELECT id FROM betreuende WHERE name = 'Trainer'").Scan(&betreuerID)
	if err := database.SaveGroupBetreuende(db, []models.Group{
		{GroupID: 1, Betreuende: []models.Betreuende{{ID: betreuerID, Name: "Trainer"}}},
	}); err != nil {
		t.Fatalf("SaveGroupBetreuende: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group")
	}
	if len(groups[0].Betreuende) != 1 {
		t.Errorf("expected 1 betreuer, got %d", len(groups[0].Betreuende))
	}
	if groups[0].Betreuende[0].Name != "Trainer" {
		t.Errorf("expected Trainer, got %q", groups[0].Betreuende[0].Name)
	}
}

// ---------------------------------------------------------------------------
// GetAllGroupIDs
// ---------------------------------------------------------------------------

func TestGetAllGroupIDs_EmptyDB_ReturnsNil(t *testing.T) {
	db := newTestDB(t)
	ids, err := database.GetAllGroupIDs(db)
	if err != nil {
		t.Fatalf("GetAllGroupIDs: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 group IDs, got %d", len(ids))
	}
}

func TestGetAllGroupIDs_ReturnsDistinctIDsSorted(t *testing.T) {
	db := newTestDB(t)
	mustInsertParticipants(t, db, participantRows(4))

	all, _ := database.GetAllTeilnehmende(db)
	// Two participants in group 3, two in group 1 (out of order to test sorting)
	if err := database.SaveGroups(db, []models.Group{
		{GroupID: 3, Teilnehmende: []models.Teilnehmende{all[0], all[1]}},
		{GroupID: 1, Teilnehmende: []models.Teilnehmende{all[2], all[3]}},
	}); err != nil {
		t.Fatalf("SaveGroups: %v", err)
	}

	ids, err := database.GetAllGroupIDs(db)
	if err != nil {
		t.Fatalf("GetAllGroupIDs: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 distinct IDs, got %d: %v", len(ids), ids)
	}
	if ids[0] != 1 || ids[1] != 3 {
		t.Errorf("expected [1 3] (sorted), got %v", ids)
	}
}

// ---------------------------------------------------------------------------
// GetStationsForReport
// ---------------------------------------------------------------------------

func TestGetStationsForReport_EmptyDB_ReturnsEmpty(t *testing.T) {
	db := newTestDB(t)
	stations, err := database.GetStationsForReport(db)
	if err != nil {
		t.Fatalf("GetStationsForReport: %v", err)
	}
	if len(stations) != 0 {
		t.Errorf("expected 0 stations, got %d", len(stations))
	}
}

func TestGetStationsForReport_StationWithoutScores(t *testing.T) {
	db := newTestDB(t)
	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"Bogenschießen"},
	})

	stations, err := database.GetStationsForReport(db)
	if err != nil {
		t.Fatalf("GetStationsForReport: %v", err)
	}
	if len(stations) != 1 {
		t.Fatalf("expected 1 station, got %d", len(stations))
	}
	if stations[0].StationName != "Bogenschießen" {
		t.Errorf("expected station Bogenschießen, got %q", stations[0].StationName)
	}
	if len(stations[0].GroupScores) != 0 {
		t.Errorf("expected 0 group scores, got %d", len(stations[0].GroupScores))
	}
}

func TestGetStationsForReport_ReturnsScoresWithStation(t *testing.T) {
	db := newTestDB(t)
	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"Station A"},
	})
	mustInsertParticipants(t, db, participantRows(2))

	var stationID int
	db.QueryRow("SELECT station_id FROM stations WHERE station_name = 'Station A'").Scan(&stationID)

	if err := database.AssignGroupStationScore(db, 1, stationID, 500); err != nil {
		t.Fatalf("AssignGroupStationScore: %v", err)
	}
	if err := database.AssignGroupStationScore(db, 2, stationID, 600); err != nil {
		t.Fatalf("AssignGroupStationScore: %v", err)
	}

	stations, err := database.GetStationsForReport(db)
	if err != nil {
		t.Fatalf("GetStationsForReport: %v", err)
	}
	if len(stations) != 1 {
		t.Fatalf("expected 1 station, got %d", len(stations))
	}
	if len(stations[0].GroupScores) != 2 {
		t.Errorf("expected 2 group scores, got %d", len(stations[0].GroupScores))
	}
}
