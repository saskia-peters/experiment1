package database_test

import (
	"testing"

	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/models"
)

// ---------------------------------------------------------------------------
// GetGroupEvaluations
// ---------------------------------------------------------------------------

func TestGetGroupEvaluations_EmptyDB_ReturnsEmpty(t *testing.T) {
	db := newTestDB(t)
	evals, err := database.GetGroupEvaluations(db)
	if err != nil {
		t.Fatalf("GetGroupEvaluations: %v", err)
	}
	if len(evals) != 0 {
		t.Errorf("expected 0 evaluations, got %d", len(evals))
	}
}

func TestGetGroupEvaluations_SumsScoresCorrectly(t *testing.T) {
	db := newTestDB(t)
	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"S1"},
		{"S2"},
	})

	rows, _ := db.Query("SELECT station_id FROM stations ORDER BY station_name")
	defer rows.Close()
	var sids []int
	for rows.Next() {
		var id int
		rows.Scan(&id)
		sids = append(sids, id)
	}

	// Group 1: 300 + 200 = 500
	database.AssignGroupStationScore(db, 1, sids[0], 300)
	database.AssignGroupStationScore(db, 1, sids[1], 200)
	// Group 2: 400
	database.AssignGroupStationScore(db, 2, sids[0], 400)

	evals, err := database.GetGroupEvaluations(db)
	if err != nil {
		t.Fatalf("GetGroupEvaluations: %v", err)
	}
	if len(evals) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(evals))
	}

	byID := make(map[int]models.GroupEvaluation)
	for _, e := range evals {
		byID[e.GroupID] = e
	}

	if byID[1].TotalScore != 500 {
		t.Errorf("group 1: expected TotalScore=500, got %d", byID[1].TotalScore)
	}
	if byID[1].StationCount != 2 {
		t.Errorf("group 1: expected StationCount=2, got %d", byID[1].StationCount)
	}
	if byID[2].TotalScore != 400 {
		t.Errorf("group 2: expected TotalScore=400, got %d", byID[2].TotalScore)
	}
	if byID[2].StationCount != 1 {
		t.Errorf("group 2: expected StationCount=1, got %d", byID[2].StationCount)
	}
}

func TestGetGroupEvaluations_RankedByTotalScoreDescending(t *testing.T) {
	db := newTestDB(t)
	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"S1"},
	})
	var sid int
	db.QueryRow("SELECT station_id FROM stations").Scan(&sid)

	database.AssignGroupStationScore(db, 1, sid, 100)
	database.AssignGroupStationScore(db, 2, sid, 500)
	database.AssignGroupStationScore(db, 3, sid, 300)

	evals, err := database.GetGroupEvaluations(db)
	if err != nil {
		t.Fatalf("GetGroupEvaluations: %v", err)
	}

	if evals[0].GroupID != 2 {
		t.Errorf("1st place: expected group 2 (500), got group %d", evals[0].GroupID)
	}
	if evals[1].GroupID != 3 {
		t.Errorf("2nd place: expected group 3 (300), got group %d", evals[1].GroupID)
	}
	if evals[2].GroupID != 1 {
		t.Errorf("3rd place: expected group 1 (100), got group %d", evals[2].GroupID)
	}
}

func TestGetGroupEvaluations_TiesBrokenByGroupIDAscending(t *testing.T) {
	db := newTestDB(t)
	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"S1"},
	})
	var sid int
	db.QueryRow("SELECT station_id FROM stations").Scan(&sid)

	database.AssignGroupStationScore(db, 5, sid, 400)
	database.AssignGroupStationScore(db, 2, sid, 400)
	database.AssignGroupStationScore(db, 8, sid, 400)

	evals, err := database.GetGroupEvaluations(db)
	if err != nil {
		t.Fatalf("GetGroupEvaluations: %v", err)
	}

	// Same score → sorted by group_id ASC
	if evals[0].GroupID != 2 {
		t.Errorf("expected group 2 first on tie, got %d", evals[0].GroupID)
	}
	if evals[1].GroupID != 5 {
		t.Errorf("expected group 5 second on tie, got %d", evals[1].GroupID)
	}
	if evals[2].GroupID != 8 {
		t.Errorf("expected group 8 third on tie, got %d", evals[2].GroupID)
	}
}

// ---------------------------------------------------------------------------
// GetOrtsverbandEvaluations
// ---------------------------------------------------------------------------

func TestGetOrtsverbandEvaluations_EmptyDB_ReturnsEmpty(t *testing.T) {
	db := newTestDB(t)
	evals, err := database.GetOrtsverbandEvaluations(db)
	if err != nil {
		t.Fatalf("GetOrtsverbandEvaluations: %v", err)
	}
	if len(evals) != 0 {
		t.Errorf("expected 0 evaluations, got %d", len(evals))
	}
}

func TestGetOrtsverbandEvaluations_AverageScoreCalculation(t *testing.T) {
	db := newTestDB(t)

	// 2 participants from Berlin in group 1 with one station score of 600.
	// The query sums each participant's group score, so:
	//   total_score = 600 (Alice) + 600 (Bob) = 1200
	//   participant_count = 2
	//   average_score = 1200 / 2 = 600
	mustInsertParticipants(t, db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", ""},
		{"Bob", "Berlin", "22", "M", ""},
	})
	all, _ := database.GetAllTeilnehmende(db)
	database.SaveGroups(db, []models.Group{
		{GroupID: 1, Teilnehmende: all},
	})

	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"S1"},
	})
	var sid int
	db.QueryRow("SELECT station_id FROM stations").Scan(&sid)
	database.AssignGroupStationScore(db, 1, sid, 600)

	evals, err := database.GetOrtsverbandEvaluations(db)
	if err != nil {
		t.Fatalf("GetOrtsverbandEvaluations: %v", err)
	}
	if len(evals) != 1 {
		t.Fatalf("expected 1 ortsverband evaluation, got %d", len(evals))
	}

	ev := evals[0]
	if ev.Ortsverband != "Berlin" {
		t.Errorf("expected Ortsverband=Berlin, got %q", ev.Ortsverband)
	}
	if ev.ParticipantCount != 2 {
		t.Errorf("expected ParticipantCount=2, got %d", ev.ParticipantCount)
	}
	// Each participant contributes their group's total score → 600 * 2 = 1200, avg = 600
	if ev.AverageScore != 600.0 {
		t.Errorf("expected AverageScore=600.0, got %f", ev.AverageScore)
	}
}

func TestGetOrtsverbandEvaluations_RankedByAverageDescending(t *testing.T) {
	db := newTestDB(t)

	mustInsertParticipants(t, db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", ""},
		{"Bob", "Hamburg", "22", "M", ""},
	})
	all, _ := database.GetAllTeilnehmende(db)
	// Put each in their own group
	database.SaveGroups(db, []models.Group{
		{GroupID: 1, Teilnehmende: []models.Teilnehmende{all[0]}},
		{GroupID: 2, Teilnehmende: []models.Teilnehmende{all[1]}},
	})

	mustInsertStations(t, db, [][]string{
		{"Stationsname"},
		{"S1"},
	})
	var sid int
	db.QueryRow("SELECT station_id FROM stations").Scan(&sid)
	database.AssignGroupStationScore(db, 1, sid, 800) // Berlin avg = 800
	database.AssignGroupStationScore(db, 2, sid, 400) // Hamburg avg = 400

	evals, err := database.GetOrtsverbandEvaluations(db)
	if err != nil {
		t.Fatalf("GetOrtsverbandEvaluations: %v", err)
	}
	if len(evals) != 2 {
		t.Fatalf("expected 2 evaluations, got %d", len(evals))
	}
	if evals[0].Ortsverband != "Berlin" {
		t.Errorf("expected Berlin first (800 avg), got %q", evals[0].Ortsverband)
	}
	if evals[1].Ortsverband != "Hamburg" {
		t.Errorf("expected Hamburg second (400 avg), got %q", evals[1].Ortsverband)
	}
}

func TestGetOrtsverbandEvaluations_NoScoresYield_EmptyResult(t *testing.T) {
	db := newTestDB(t)
	mustInsertParticipants(t, db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", ""},
	})
	all, _ := database.GetAllTeilnehmende(db)
	database.SaveGroups(db, []models.Group{
		{GroupID: 1, Teilnehmende: all},
	})

	// No station scores assigned → no ortsverband evaluation rows
	evals, err := database.GetOrtsverbandEvaluations(db)
	if err != nil {
		t.Fatalf("GetOrtsverbandEvaluations: %v", err)
	}
	if len(evals) != 0 {
		t.Errorf("expected 0 evaluations without scores, got %d", len(evals))
	}
}
