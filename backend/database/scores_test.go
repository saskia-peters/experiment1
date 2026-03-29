package database_test

import (
	"testing"

	"THW-JugendOlympiade/backend/database"
)

// ---------------------------------------------------------------------------
// AssignGroupStationScore
// ---------------------------------------------------------------------------

func TestAssignGroupStationScore_InsertsNewScore(t *testing.T) {
	db := newTestDB(t)
	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"Station A"},
	})

	var stationID int
	db.QueryRow("SELECT station_id FROM stations").Scan(&stationID)

	if err := database.AssignGroupStationScore(db, 1, stationID, 500); err != nil {
		t.Fatalf("AssignGroupStationScore: %v", err)
	}

	var score int
	db.QueryRow(
		"SELECT score FROM group_station_scores WHERE group_id = 1 AND station_id = ?", stationID,
	).Scan(&score)
	if score != 500 {
		t.Errorf("expected score 500, got %d", score)
	}
}

func TestAssignGroupStationScore_UpdatesExistingScore(t *testing.T) {
	db := newTestDB(t)
	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"Station A"},
	})

	var stationID int
	db.QueryRow("SELECT station_id FROM stations").Scan(&stationID)

	// Insert initial score
	if err := database.AssignGroupStationScore(db, 1, stationID, 300); err != nil {
		t.Fatalf("first AssignGroupStationScore: %v", err)
	}
	// Update to new score
	if err := database.AssignGroupStationScore(db, 1, stationID, 700); err != nil {
		t.Fatalf("second AssignGroupStationScore: %v", err)
	}

	// Only one row should exist
	var count int
	db.QueryRow(
		"SELECT COUNT(*) FROM group_station_scores WHERE group_id = 1 AND station_id = ?", stationID,
	).Scan(&count)
	if count != 1 {
		t.Errorf("expected 1 score row after update, got %d", count)
	}

	var score int
	db.QueryRow(
		"SELECT score FROM group_station_scores WHERE group_id = 1 AND station_id = ?", stationID,
	).Scan(&score)
	if score != 700 {
		t.Errorf("expected updated score 700, got %d", score)
	}
}

func TestAssignGroupStationScore_AcceptsZeroScore(t *testing.T) {
	db := newTestDB(t)
	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"Station A"},
	})

	var stationID int
	db.QueryRow("SELECT station_id FROM stations").Scan(&stationID)

	if err := database.AssignGroupStationScore(db, 1, stationID, 0); err != nil {
		t.Fatalf("AssignGroupStationScore with score=0: %v", err)
	}

	var score int
	db.QueryRow(
		"SELECT score FROM group_station_scores WHERE group_id = 1 AND station_id = ?", stationID,
	).Scan(&score)
	if score != 0 {
		t.Errorf("expected score 0, got %d", score)
	}
}

func TestAssignGroupStationScore_MultipleGroupsAtSameStation(t *testing.T) {
	db := newTestDB(t)
	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"Station A"},
	})

	var stationID int
	db.QueryRow("SELECT station_id FROM stations").Scan(&stationID)

	scores := map[int]int{1: 100, 2: 200, 3: 300}
	for groupID, score := range scores {
		if err := database.AssignGroupStationScore(db, groupID, stationID, score); err != nil {
			t.Fatalf("AssignGroupStationScore(group=%d): %v", groupID, err)
		}
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM group_station_scores WHERE station_id = ?", stationID).Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 score rows, got %d", count)
	}
}

func TestAssignGroupStationScore_MultipleStationsForSameGroup(t *testing.T) {
	db := newTestDB(t)
	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"Station A"},
		{"Station B"},
		{"Station C"},
	})

	rows, _ := db.Query("SELECT station_id FROM stations ORDER BY station_id")
	defer rows.Close()
	var stationIDs []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		stationIDs = append(stationIDs, id)
	}

	for _, sid := range stationIDs {
		if err := database.AssignGroupStationScore(db, 1, sid, 200); err != nil {
			t.Fatalf("AssignGroupStationScore(station=%d): %v", sid, err)
		}
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM group_station_scores WHERE group_id = 1").Scan(&count)
	if count != 3 {
		t.Errorf("expected 3 station scores for group 1, got %d", count)
	}
}
