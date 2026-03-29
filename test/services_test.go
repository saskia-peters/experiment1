package test

import (
	"testing"

	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/services"
)

// testSvcGroupSize is the fixed group size used by the services tests.
// It is independent of the config default so tests remain stable when
// the operator changes their configuration.
const testSvcGroupSize = 8

// TestCreateBalancedGroups_EmptyDB verifies that no groups are created when there
// are no participants in the database.
func TestCreateBalancedGroups_EmptyDB(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	if _, err := services.CreateBalancedGroups(db, testSvcGroupSize); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ids, err := database.GetAllGroupIDs(db)
	if err != nil {
		t.Fatalf("GetAllGroupIDs failed: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected no groups for empty DB, got %d", len(ids))
	}
}

// TestCreateBalancedGroups_GroupCountCorrect checks that the right number of groups
// is created for various participant counts.
func TestCreateBalancedGroups_GroupCountCorrect(t *testing.T) {
	tests := []struct {
		name            string
		numParticipants int
		expectedGroups  int
	}{
		{"1 participant → 1 group", 1, 1},
		{"8 participants → 1 group (exactly full)", 8, 1},
		{"9 participants → 2 groups", 9, 2},
		{"16 participants → 2 groups", 16, 2},
		{"17 participants → 3 groups", 17, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := setupFullTestDB(t)
			defer teardownTestDB(t, db)

			rows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
			for i := 1; i <= tt.numParticipants; i++ {
				rows = append(rows, []string{"P", "Berlin", "20", "M", ""})
			}
			if err := database.InsertData(db, rows); err != nil {
				t.Fatalf("InsertData failed: %v", err)
			}

			if _, err := services.CreateBalancedGroups(db, testSvcGroupSize); err != nil {
				t.Fatalf("CreateBalancedGroups failed: %v", err)
			}

			ids, err := database.GetAllGroupIDs(db)
			if err != nil {
				t.Fatalf("GetAllGroupIDs failed: %v", err)
			}
			if len(ids) != tt.expectedGroups {
				t.Errorf("expected %d groups, got %d", tt.expectedGroups, len(ids))
			}
		})
	}
}

// TestCreateBalancedGroups_NoGroupExceedsMaxSize ensures the distribution algorithm
// never puts more than MaxGroupSize participants into a single group.
func TestCreateBalancedGroups_NoGroupExceedsMaxSize(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	rows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	ortsverbands := []string{"Berlin", "Hamburg", "München", "Köln"}
	geschlechts := []string{"M", "W"}
	for i := 1; i <= 25; i++ {
		rows = append(rows, []string{
			"P", ortsverbands[i%len(ortsverbands)], "20", geschlechts[i%len(geschlechts)], "",
		})
	}
	if err := database.InsertData(db, rows); err != nil {
		t.Fatalf("InsertData failed: %v", err)
	}

	if _, err := services.CreateBalancedGroups(db, testSvcGroupSize); err != nil {
		t.Fatalf("CreateBalancedGroups failed: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport failed: %v", err)
	}

	for _, g := range groups {
		if len(g.Teilnehmende) > testSvcGroupSize {
			t.Errorf("group %d has %d participants, exceeds max %d",
				g.GroupID, len(g.Teilnehmende), testSvcGroupSize)
		}
	}
}

// TestCreateBalancedGroups_PreGroupMembersStayTogether verifies that participants
// sharing the same PreGroup label are placed into the same group.
func TestCreateBalancedGroups_PreGroupMembersStayTogether(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 3 participants with PreGroup "Alpha", 2 without
	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", "Alpha"},
		{"Bob", "Hamburg", "22", "M", "Alpha"},
		{"Carol", "München", "28", "W", "Alpha"},
		{"Dave", "Köln", "20", "M", ""},
		{"Eve", "Berlin", "24", "W", ""},
	}); err != nil {
		t.Fatalf("InsertData failed: %v", err)
	}

	if _, err := services.CreateBalancedGroups(db, testSvcGroupSize); err != nil {
		t.Fatalf("CreateBalancedGroups failed: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport failed: %v", err)
	}

	// All 5 participants must be grouped
	total := 0
	for _, g := range groups {
		total += len(g.Teilnehmende)
	}
	if total != 5 {
		t.Errorf("expected 5 participants in groups, got %d", total)
	}

	// Find which group contains Alice (teilnehmer_id=1 from InsertData, row index 1)
	// and verify Bob and Carol are in the same group.
	groupOfAlice := -1
	for _, g := range groups {
		for _, p := range g.Teilnehmende {
			if p.Name == "Alice" {
				groupOfAlice = g.GroupID
				break
			}
		}
		if groupOfAlice != -1 {
			break
		}
	}
	if groupOfAlice == -1 {
		t.Fatal("Alice not found in any group")
	}

	for _, name := range []string{"Bob", "Carol"} {
		found := false
		for _, g := range groups {
			if g.GroupID != groupOfAlice {
				continue
			}
			for _, p := range g.Teilnehmende {
				if p.Name == name {
					found = true
					break
				}
			}
		}
		if !found {
			t.Errorf("%s (PreGroup=Alpha) should be in the same group as Alice (group %d)", name, groupOfAlice)
		}
	}
}

// TestCreateBalancedGroups_WithBetreuende verifies that betreuende are distributed
// across groups and saved to the database.
func TestCreateBalancedGroups_WithBetreuende(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 4 participants split across 2 Ortsverbands; 1 trainer per Ortsverband
	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", ""},
		{"Bob", "Berlin", "22", "M", ""},
		{"Carol", "Hamburg", "28", "W", ""},
		{"Dave", "Hamburg", "20", "M", ""},
	}); err != nil {
		t.Fatalf("InsertData failed: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Trainer Berlin", "Berlin", "ja"},
		{"Trainer Hamburg", "Hamburg", "ja"},
	}); err != nil {
		t.Fatalf("InsertBetreuende failed: %v", err)
	}

	if _, err := services.CreateBalancedGroups(db, testSvcGroupSize); err != nil {
		t.Fatalf("CreateBalancedGroups failed: %v", err)
	}

	// Both trainers should be assigned (saved to gruppe_betreuende)
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM gruppe_betreuende").Scan(&count); err != nil {
		t.Fatalf("count query failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 betreuende assignments, got %d", count)
	}
}

// TestCreateBalancedGroups_ReroutingClearsOldGroups verifies that calling
// CreateBalancedGroups twice replaces the previous group assignments.
func TestCreateBalancedGroups_ReroutingClearsOldGroups(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", ""},
		{"Bob", "Hamburg", "22", "M", ""},
	}); err != nil {
		t.Fatalf("InsertData failed: %v", err)
	}

	// First run
	if _, err := services.CreateBalancedGroups(db, testSvcGroupSize); err != nil {
		t.Fatalf("first CreateBalancedGroups failed: %v", err)
	}

	var countAfterFirst int
	if err := db.QueryRow("SELECT COUNT(*) FROM gruppe").Scan(&countAfterFirst); err != nil {
		t.Fatalf("count query failed: %v", err)
	}

	// Second run — should replace, not accumulate
	if _, err := services.CreateBalancedGroups(db, testSvcGroupSize); err != nil {
		t.Fatalf("second CreateBalancedGroups failed: %v", err)
	}

	var countAfterSecond int
	if err := db.QueryRow("SELECT COUNT(*) FROM gruppe").Scan(&countAfterSecond); err != nil {
		t.Fatalf("count query failed: %v", err)
	}

	if countAfterSecond != countAfterFirst {
		t.Errorf("expected same gruppe count on second run (%d), got %d",
			countAfterFirst, countAfterSecond)
	}
}

// TestCreateBalancedGroups_PreGroupExceedsMaxSize_ReturnsError verifies that an
// error is returned when a PreGroup tag has more members than maxGroupSize.
func TestCreateBalancedGroups_PreGroupExceedsMaxSize_ReturnsError(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// Insert 3 participants all sharing PreGroup "Trio" with maxGroupSize=2.
	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", "Trio"},
		{"Bob", "Hamburg", "22", "M", "Trio"},
		{"Carol", "München", "20", "W", "Trio"},
	}); err != nil {
		t.Fatalf("InsertData failed: %v", err)
	}

	_, err := services.CreateBalancedGroups(db, 2)
	if err == nil {
		t.Fatal("expected error when PreGroup size exceeds maxGroupSize, got nil")
	}
}

// TestCreateBalancedGroups_AllParticipantsAssigned verifies that every participant
// inserted into the database is placed into exactly one group.
func TestCreateBalancedGroups_AllParticipantsAssigned(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	const n = 19 // odd number to stress the last group
	rows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 1; i <= n; i++ {
		rows = append(rows, []string{"P", "Berlin", "20", "M", ""})
	}
	if err := database.InsertData(db, rows); err != nil {
		t.Fatalf("InsertData failed: %v", err)
	}

	if _, err := services.CreateBalancedGroups(db, testSvcGroupSize); err != nil {
		t.Fatalf("CreateBalancedGroups failed: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport failed: %v", err)
	}

	total := 0
	for _, g := range groups {
		total += len(g.Teilnehmende)
	}
	if total != n {
		t.Errorf("expected all %d participants assigned, got %d", n, total)
	}
}

// TestCreateBalancedGroups_FewerLicensedDriversThanGroups_ReturnsWarning checks
// that a non-empty warning is returned when there are not enough licensed drivers
// to cover every group with one.
func TestCreateBalancedGroups_FewerLicensedDriversThanGroups_ReturnsWarning(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 9 participants → 2 groups (maxGroupSize=8), but only 1 licensed driver.
	rows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 1; i <= 9; i++ {
		rows = append(rows, []string{"P", "Berlin", "20", "M", ""})
	}
	if err := database.InsertData(db, rows); err != nil {
		t.Fatalf("InsertData failed: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Trainer A", "Berlin", "ja"}, // only one licensed driver for two groups
	}); err != nil {
		t.Fatalf("InsertBetreuende failed: %v", err)
	}

	warning, err := services.CreateBalancedGroups(db, testSvcGroupSize)
	if err != nil {
		t.Fatalf("CreateBalancedGroups failed: %v", err)
	}
	if warning == "" {
		t.Error("expected a non-empty warning when licensed drivers < number of groups")
	}
}
