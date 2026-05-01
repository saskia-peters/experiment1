package test

import (
	"strings"
	"testing"

	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/config"
	"THW-JugendOlympiade/backend/services"
)

// testSvcGroupSize is the fixed group size used by the services tests.
// It is independent of the config default so tests remain stable when
// the operator changes their configuration.
const testSvcGroupSize = 8

// svcCfg builds a minimal config.Config for services tests.
// The modus is always "Klassisch" so existing tests continue to exercise the
// original no-vehicle and vehicle-first code paths unchanged.
func svcCfg(maxGroupSize, minGroupSize int) config.Config {
	cfg := config.Default()
	cfg.Verteilung.Verteilungsmodus = "Klassisch"
	cfg.Gruppen.MaxGroesse = maxGroupSize
	cfg.Gruppen.MinGroesse = minGroupSize
	return cfg
}

// TestCreateBalancedGroups_EmptyDB verifies that no groups are created when there
// are no participants in the database.
func TestCreateBalancedGroups_EmptyDB(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
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

			if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
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

	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
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

	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
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

	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
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
	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
		t.Fatalf("first CreateBalancedGroups failed: %v", err)
	}

	var countAfterFirst int
	if err := db.QueryRow("SELECT COUNT(*) FROM gruppe").Scan(&countAfterFirst); err != nil {
		t.Fatalf("count query failed: %v", err)
	}

	// Second run — should replace, not accumulate
	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
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

	_, err := services.CreateBalancedGroups(db, svcCfg(2, 0))
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

	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
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

	warning, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0))
	if err != nil {
		t.Fatalf("CreateBalancedGroups failed: %v", err)
	}
	if warning == "" {
		t.Error("expected a non-empty warning when licensed drivers < number of groups")
	}
}

// TestCreateBalancedGroups_DriverAppearsInBetreuendeNotDoubled verifies that when
// vehicles are present, the vehicle's Fahrer (driver) is listed in the group's
// Betreuende section exactly once, and the seat capacity check does not
// double-count that person (driver seat is included in Sitzplaetze AND the
// driver is counted once in Betreuende, not twice).
func TestCreateBalancedGroups_DriverAppearsInBetreuendeNotDoubled(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 4 participants, 1 driver (also betreuende), 1 non-driver betreuende,
	// 1 vehicle with 10 seats (includes driver seat).
	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"Alice", "Berlin", "25", "W", ""},
		{"Bob", "Berlin", "22", "M", ""},
		{"Carol", "Berlin", "28", "W", ""},
		{"Dave", "Berlin", "20", "M", ""},
	}); err != nil {
		t.Fatalf("InsertData failed: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Driver Max", "Berlin", "ja"}, // also the vehicle driver
		{"Helper Anna", "Berlin", "nein"},
	}); err != nil {
		t.Fatalf("InsertBetreuende failed: %v", err)
	}
	// Vehicle whose FahrerName matches "Driver Max" exactly.
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"Bus 1", "Berlin", "BLN-1", "Driver Max", "10"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge failed: %v", err)
	}

	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
		t.Fatalf("CreateBalancedGroups failed: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport failed: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]

	// The driver must appear in the Betreuende list exactly once.
	driverCount := 0
	for _, b := range g.Betreuende {
		if b.Name == "Driver Max" {
			driverCount++
		}
	}
	if driverCount != 1 {
		t.Errorf("expected Driver Max to appear exactly once in Betreuende, got %d times", driverCount)
	}

	// Both betreuende (driver + helper) must be present.
	if len(g.Betreuende) != 2 {
		t.Errorf("expected 2 Betreuende (driver + helper), got %d", len(g.Betreuende))
	}

	// Seat capacity check: totalPeople = Teilnehmende + Betreuende (driver counted once).
	// The vehicle has 10 seats (including driver's seat), so 4+2=6 people fit fine.
	totalPeople := len(g.Teilnehmende) + len(g.Betreuende)
	totalSeats := 0
	for _, f := range g.Fahrzeuge {
		totalSeats += f.Sitzplaetze
	}
	if totalPeople > totalSeats {
		t.Errorf("seat capacity exceeded: %d people, %d seats — driver was likely counted twice",
			totalPeople, totalSeats)
	}
}

// ─── Vehicle-first algorithm tests ────────────────────────────────────────────

// TestCreateBalancedGroups_VehicleFirst_GroupCountEqualsVehicleCount verifies
// that the number of saved groups equals the number of eligible vehicles.
func TestCreateBalancedGroups_VehicleFirst_GroupCountEqualsVehicleCount(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"TN1", "A", "14", "M", ""}, {"TN2", "B", "14", "W", ""}, {"TN3", "C", "15", "M", ""},
		{"TN4", "D", "14", "M", ""}, {"TN5", "E", "14", "W", ""}, {"TN6", "F", "15", "M", ""},
		{"TN7", "G", "14", "M", ""}, {"TN8", "H", "14", "W", ""}, {"TN9", "I", "15", "M", ""},
	}); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"Bus 1", "A", "", "", "8"},
		{"Bus 2", "B", "", "", "8"},
		{"Bus 3", "C", "", "", "8"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 3 {
		t.Errorf("expected 3 groups (= vehicle count), got %d", len(groups))
	}
	totalTN := 0
	for _, g := range groups {
		totalTN += len(g.Teilnehmende)
	}
	if totalTN != 9 {
		t.Errorf("expected all 9 TN assigned, got %d", totalTN)
	}
}

// TestCreateBalancedGroups_VehicleFirst_UnusedVehiclesReported verifies that
// groups receiving no Teilnehmende are not saved and an informational warning
// is emitted.
func TestCreateBalancedGroups_VehicleFirst_UnusedVehiclesReported(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 2 TN but 3 vehicles — diversity scoring spreads 1 TN to each of the first
	// two groups; the third group remains empty and must not be saved.
	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"TN1", "Alpha", "14", "M", ""},
		{"TN2", "Beta", "14", "W", ""},
	}); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"Van 1", "Alpha", "", "", "10"},
		{"Van 2", "Beta", "", "", "10"},
		{"Van 3", "Gamma", "", "", "10"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	warning, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0))
	if err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	for _, g := range groups {
		if len(g.Teilnehmende) == 0 {
			t.Errorf("group %d has 0 Teilnehmende but was saved", g.GroupID)
		}
	}
	if !strings.Contains(warning, "Ungenutzte Fahrzeuge") {
		t.Errorf("expected warning about unused vehicles, got: %q", warning)
	}
}

// TestCreateBalancedGroups_VehicleFirst_AllTNFit_NoWarning checks that when
// total effective capacity exceeds participant count, no overload warning is
// emitted.
func TestCreateBalancedGroups_VehicleFirst_AllTNFit_NoWarning(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// V1 (5-seat): effectiveCap=min(8,5)=5. V2 (10-seat): effectiveCap=min(8,10)=8.
	// 8 TN — fit without triggering overload.
	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"TN1", "A", "14", "M", ""}, {"TN2", "B", "14", "W", ""},
		{"TN3", "C", "15", "M", ""}, {"TN4", "D", "14", "M", ""},
		{"TN5", "E", "14", "W", ""}, {"TN6", "F", "15", "M", ""},
		{"TN7", "G", "14", "M", ""}, {"TN8", "H", "14", "W", ""},
	}); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"Bus A", "A", "", "", "5"},
		{"Bus B", "B", "", "", "10"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	warning, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0))
	if err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}
	if strings.Contains(warning, "Kapazitätsengpass") {
		t.Errorf("expected no overload warning, got: %q", warning)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	totalTN := 0
	for _, g := range groups {
		totalTN += len(g.Teilnehmende)
	}
	if totalTN != 8 {
		t.Errorf("expected 8 TN placed, got %d", totalTN)
	}
}

// TestCreateBalancedGroups_VehicleFirst_VehicleCapDominatesOverMaxGroupSize
// verifies that when seats−betreuende < maxGroupSize, the vehicle seat count
// is the binding cap.
func TestCreateBalancedGroups_VehicleFirst_VehicleCapDominatesOverMaxGroupSize(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 1 vehicle (4-seat), 1 driver → effectiveCap = min(8, 4-1) = 3.
	// 3 TN exactly fill it; no overflow.
	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"TN1", "A", "14", "M", ""},
		{"TN2", "B", "14", "W", ""},
		{"TN3", "C", "15", "M", ""},
	}); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Driver", "A", "ja"},
	}); err != nil {
		t.Fatalf("InsertBetreuende: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"SmallBus", "A", "", "Driver", "4"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	warning, err := services.CreateBalancedGroups(db, svcCfg(8, 0))
	if err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}
	if strings.Contains(warning, "Kapazitätsengpass") {
		t.Errorf("expected no overload warning for 3 TN in a 4-seat vehicle (1 driver), got: %q", warning)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Teilnehmende) != 3 {
		t.Errorf("expected 3 TN placed, got %d", len(groups[0].Teilnehmende))
	}
}

// TestCreateBalancedGroups_VehicleFirst_PlusOneApplies_VehicleHasHeadroom checks
// that surplus vehicle seats beyond maxGroupSize absorb overflow via +1 exception.
func TestCreateBalancedGroups_VehicleFirst_PlusOneApplies_VehicleHasHeadroom(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 2 vehicles (9-seat each), 1 driver each, maxGroupSize=6.
	// effectiveCap = min(6, 9-1) = 6; total cap = 12.
	// 13 TN → 1 overflow. Headroom = (9-1)−6 = 2 per vehicle → 4 total ≥ 1 → +1 applied.
	tnRows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 0; i < 13; i++ {
		tnRows = append(tnRows, []string{"TN", "X", "14", "M", ""})
	}
	if err := database.InsertData(db, tnRows); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Driver1", "A", "ja"},
		{"Driver2", "B", "ja"},
	}); err != nil {
		t.Fatalf("InsertBetreuende: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"Big A", "A", "", "Driver1", "9"},
		{"Big B", "B", "", "Driver2", "9"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	warning, err := services.CreateBalancedGroups(db, svcCfg(6, 0))
	if err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}
	if !strings.Contains(warning, "+1-Ausnahme") {
		t.Errorf("expected +1-Ausnahme warning, got: %q", warning)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	totalTN := 0
	for _, g := range groups {
		totalTN += len(g.Teilnehmende)
	}
	if totalTN != 13 {
		t.Errorf("expected all 13 TN placed, got %d", totalTN)
	}
}

// TestCreateBalancedGroups_VehicleFirst_PlusOneNotApplicable_VehicleIsConstraint
// checks that when vehicle seats − driver <= maxGroupSize, the +1 exception does
// not apply and overflow falls back to least-full placement.
func TestCreateBalancedGroups_VehicleFirst_PlusOneNotApplicable_VehicleIsConstraint(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 2 vehicles (6-seat each), 1 driver each, maxGroupSize=8.
	// effectiveCap = min(8, 6-1) = 5. Total TN cap = 10.
	// 11 TN → 1 overflow. seats−driver=5 not > maxGroupSize=8 → no +1.
	tnRows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 0; i < 11; i++ {
		tnRows = append(tnRows, []string{"TN", "X", "14", "M", ""})
	}
	if err := database.InsertData(db, tnRows); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Driver1", "A", "ja"},
		{"Driver2", "B", "ja"},
	}); err != nil {
		t.Fatalf("InsertBetreuende: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"Mid A", "A", "", "Driver1", "6"},
		{"Mid B", "B", "", "Driver2", "6"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	warning, err := services.CreateBalancedGroups(db, svcCfg(8, 0))
	if err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}
	if strings.Contains(warning, "+1-Ausnahme") {
		t.Errorf("+1-Ausnahme must NOT apply when vehicle is the constraint, got: %q", warning)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	totalTN := 0
	for _, g := range groups {
		totalTN += len(g.Teilnehmende)
	}
	if totalTN != 11 {
		t.Errorf("expected all 11 TN placed (fallback), got %d", totalTN)
	}
}

// TestCreateBalancedGroups_VehicleFirst_MultiplePreGroupsSameVehicle verifies
// that two PreGroups can both be assigned to the larger vehicle when the smaller
// vehicle has too few seats to fit either.
func TestCreateBalancedGroups_VehicleFirst_MultiplePreGroupsSameVehicle(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// V1 (10-seat): effectiveCap=8. V2 (2-seat): effectiveCap=2.
	// PreGroup A (3 TN) and B (3 TN): V2 can't fit either → both go to V1.
	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"A1", "X", "14", "M", "A"}, {"A2", "X", "14", "W", "A"}, {"A3", "X", "15", "M", "A"},
		{"B1", "Y", "14", "M", "B"}, {"B2", "Y", "14", "W", "B"}, {"B3", "Y", "15", "M", "B"},
	}); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"BigBus", "X", "", "", "10"},
		{"Tiny", "Y", "", "", "2"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	_, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	for _, g := range groups {
		if len(g.Teilnehmende) == 0 {
			t.Errorf("group %d has 0 TN but was saved", g.GroupID)
		}
	}
	totalTN := 0
	for _, g := range groups {
		totalTN += len(g.Teilnehmende)
	}
	if totalTN != 6 {
		t.Errorf("expected all 6 TN placed, got %d", totalTN)
	}
}

// TestCreateBalancedGroups_VehicleFirst_PreGroupTooLargeForAnyVehicle verifies
// that an error is returned when a PreGroup's size exceeds every vehicle's
// effectiveCap.
func TestCreateBalancedGroups_VehicleFirst_PreGroupTooLargeForAnyVehicle(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 2 vehicles (3-seat), 1 driver each → effectiveCap = min(8, 3-1) = 2.
	// PreGroup of 3 TN → 3 > 2 for every vehicle → error.
	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"PG1", "A", "14", "M", "G"}, {"PG2", "A", "14", "W", "G"}, {"PG3", "A", "15", "M", "G"},
	}); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"DrvA", "A", "ja"},
		{"DrvB", "B", "ja"},
	}); err != nil {
		t.Fatalf("InsertBetreuende: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"Tiny1", "A", "", "DrvA", "3"},
		{"Tiny2", "B", "", "DrvB", "3"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	_, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0))
	if err == nil {
		t.Fatal("expected error for PreGroup that cannot fit in any vehicle, got nil")
	}
}

// TestCreateBalancedGroups_VehicleFirst_InsufficientTotalCapacity checks that
// a capacity-overload warning is emitted when total vehicle seats are fewer than
// total headcount.
func TestCreateBalancedGroups_VehicleFirst_InsufficientTotalCapacity(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 2 vehicles (4-seat), 1 driver each → 8 seats; 10 TN + 2 drivers = 12 people.
	tnRows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 0; i < 10; i++ {
		tnRows = append(tnRows, []string{"TN", "X", "14", "M", ""})
	}
	if err := database.InsertData(db, tnRows); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"DrvA", "A", "ja"},
		{"DrvB", "B", "ja"},
	}); err != nil {
		t.Fatalf("InsertBetreuende: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"BusA", "A", "", "DrvA", "4"},
		{"BusB", "B", "", "DrvB", "4"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	warning, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0))
	if err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}
	if !strings.Contains(warning, "Kapazitätsengpass") {
		t.Errorf("expected Kapazitätsengpass warning, got: %q", warning)
	}
}

// TestCreateBalancedGroups_VehicleFirst_MinGroupSize_SmallVehicleExcluded
// verifies that vehicles with seats−1 < minGroupSize are excluded and reported.
func TestCreateBalancedGroups_VehicleFirst_MinGroupSize_SmallVehicleExcluded(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// SmallBus: seats-1=4 < minGroupSize=5 → excluded.
	// BigBus:   seats-1=7 ≥ 5 → eligible; effectiveCap=min(8,8-1)=7.
	// 7 TN → all fit in BigBus group.
	tnRows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 0; i < 7; i++ {
		tnRows = append(tnRows, []string{"TN", "X", "14", "M", ""})
	}
	if err := database.InsertData(db, tnRows); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"SmallBus", "A", "", "", "5"},
		{"BigBus", "B", "", "", "8"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	warning, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 5))
	if err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}
	if !strings.Contains(warning, "Ausgeschlossene Fahrzeuge") {
		t.Errorf("expected excluded-vehicles warning, got: %q", warning)
	}
	if !strings.Contains(warning, "SmallBus") {
		t.Errorf("expected SmallBus in excluded warning, got: %q", warning)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 1 {
		t.Errorf("expected 1 group (only BigBus eligible), got %d", len(groups))
	}
	if len(groups[0].Teilnehmende) != 7 {
		t.Errorf("expected 7 TN in the eligible group, got %d", len(groups[0].Teilnehmende))
	}
}

// TestCreateBalancedGroups_VehicleFirst_MinGroupSize_AllExcluded verifies that
// an error is returned when minGroupSize excludes every vehicle.
func TestCreateBalancedGroups_VehicleFirst_MinGroupSize_AllExcluded(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// Both vehicles: seats-1=3 < minGroupSize=5 → both excluded.
	tnRows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 0; i < 6; i++ {
		tnRows = append(tnRows, []string{"TN", "X", "14", "M", ""})
	}
	if err := database.InsertData(db, tnRows); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"TinyA", "A", "", "", "4"},
		{"TinyB", "B", "", "", "4"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	_, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 5))
	if err == nil {
		t.Fatal("expected error when all vehicles are excluded, got nil")
	}
	if !strings.Contains(err.Error(), "alle Fahrzeuge") {
		t.Errorf("expected 'alle Fahrzeuge' in error, got: %v", err)
	}
}

// TestCreateBalancedGroups_VehicleFirst_MinGroupSize_Disabled verifies that
// minGroupSize=0 treats all vehicles as eligible regardless of seat count.
func TestCreateBalancedGroups_VehicleFirst_MinGroupSize_Disabled(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// V1 (5-seat), V2 (8-seat) — both eligible when minGroupSize=0.
	tnRows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 0; i < 7; i++ {
		tnRows = append(tnRows, []string{"TN", "X", "14", "M", ""})
	}
	if err := database.InsertData(db, tnRows); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"SmallBus", "A", "", "", "5"},
		{"BigBus", "B", "", "", "8"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	warning, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0))
	if err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}
	if strings.Contains(warning, "Ausgeschlossene Fahrzeuge") {
		t.Errorf("expected no excluded-vehicles warning with minGroupSize=0, got: %q", warning)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 2 {
		t.Errorf("expected 2 groups (both vehicles eligible), got %d", len(groups))
	}
	totalTN := 0
	for _, g := range groups {
		totalTN += len(g.Teilnehmende)
	}
	if totalTN != 7 {
		t.Errorf("expected 7 TN placed, got %d", totalTN)
	}
}

// ─── No-double-assignment tests ───────────────────────────────────────────────

// TestCreateBalancedGroups_NoBetreuendeAssignedTwice_SameGroup verifies that
// no Betreuende appears more than once in the same group's Betreuende list.
func TestCreateBalancedGroups_NoBetreuendeAssignedTwice_SameGroup(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 1 vehicle, 1 driver who is also in the Betreuende list, plus 1 extra Betreuende.
	if err := database.InsertData(db, [][]string{
		{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"},
		{"TN1", "A", "14", "M", ""}, {"TN2", "B", "14", "W", ""},
		{"TN3", "C", "15", "M", ""}, {"TN4", "D", "14", "M", ""},
		{"TN5", "E", "14", "W", ""}, {"TN6", "F", "15", "M", ""},
	}); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Driver One", "A", "ja"},
		{"Helper Two", "A", "nein"},
	}); err != nil {
		t.Fatalf("InsertBetreuende: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"Bus A", "A", "", "Driver One", "9"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	for _, g := range groups {
		seen := make(map[int]int) // betreuende.ID → count
		for _, b := range g.Betreuende {
			seen[b.ID]++
			if seen[b.ID] > 1 {
				t.Errorf("group %d: Betreuende %q (ID %d) appears %d times in same group",
					g.GroupID, b.Name, b.ID, seen[b.ID])
			}
		}
	}
}

// TestCreateBalancedGroups_NoBetreuendeAssignedToMultipleGroups verifies that
// no Betreuende appears in more than one group.
func TestCreateBalancedGroups_NoBetreuendeAssignedToMultipleGroups(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 2 vehicles, 2 drivers, 2 extra Betreuende; 12 TN across both groups.
	tnRows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 0; i < 12; i++ {
		tnRows = append(tnRows, []string{"TN", "X", "14", "M", ""})
	}
	if err := database.InsertData(db, tnRows); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Driver Alpha", "A", "ja"},
		{"Driver Beta", "B", "ja"},
		{"Helper One", "A", "nein"},
		{"Helper Two", "B", "nein"},
	}); err != nil {
		t.Fatalf("InsertBetreuende: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"Bus 1", "A", "", "Driver Alpha", "9"},
		{"Bus 2", "B", "", "Driver Beta", "9"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	if _, err := services.CreateBalancedGroups(db, svcCfg(testSvcGroupSize, 0)); err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}

	globalSeen := make(map[int]int) // betreuende.ID → groupID of first assignment
	for _, g := range groups {
		for _, b := range g.Betreuende {
			if prev, exists := globalSeen[b.ID]; exists {
				t.Errorf("Betreuende %q (ID %d) assigned to both group %d and group %d",
					b.Name, b.ID, prev, g.GroupID)
			} else {
				globalSeen[b.ID] = g.GroupID
			}
		}
	}
}

// TestCreateBalancedGroups_ReliefMovesPersonFromOverloadedGroup verifies that
// Phase 3c moves a Teilnehmende from an overloaded group (headcount > seats)
// to a group that still has spare seats, and that the relief is reported in the
// warning string.
func TestCreateBalancedGroups_ReliefMovesPersonFromOverloadedGroup(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// Setup:
	//   SmallBus (OV A, 5 seats, driver "Driver1") → Group 1
	//   BigBus   (OV B, 7 seats, driver "Driver2") → Group 2
	//
	// 8 TN, all OV "X" (no PreGroup).
	// Driver1/Driver2 are pre-assigned as vehicle drivers.
	// Helper (OV A, unlicensed) is NOT a driver → Phase 3 places her in Group 1
	// (same OV as Driver1's licensed driver) → Group 1 becomes 1+1+4 = 6 > 5 seats.
	//
	// Phase 3c must then move 1 TN from Group 1 to Group 2 to relieve the overload.
	tnRows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 0; i < 8; i++ {
		tnRows = append(tnRows, []string{"TN", "X", "14", "M", ""})
	}
	if err := database.InsertData(db, tnRows); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Driver1", "A", "ja"},
		{"Driver2", "B", "ja"},
		{"Helper", "A", "nein"},
	}); err != nil {
		t.Fatalf("InsertBetreuende: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"SmallBus", "A", "", "Driver1", "5"},
		{"BigBus", "B", "", "Driver2", "7"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	_, err := services.CreateBalancedGroups(db, svcCfg(7, 0))
	if err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// After relief no group should be overloaded.
	seatsByGroup := map[int]int{1: 5, 2: 7}
	for _, g := range groups {
		seats, ok := seatsByGroup[g.GroupID]
		if !ok {
			continue
		}
		total := len(g.Teilnehmende) + len(g.Betreuende)
		if total > seats {
			t.Errorf("group %d still overloaded: %d people, %d seats", g.GroupID, total, seats)
		}
	}

	// Total TN count must be preserved (no one lost).
	totalTN := 0
	for _, g := range groups {
		totalTN += len(g.Teilnehmende)
	}
	if totalTN != 8 {
		t.Errorf("expected 8 TN total after relief, got %d", totalTN)
	}
}

// TestCreateBalancedGroups_NonDriverBetreuendeDoNotReduceTNCapacity verifies
// that non-driver Betreuende assigned after participants do not reduce the
// number of TN placed (i.e. they are assigned after TN, not before).
func TestCreateBalancedGroups_NonDriverBetreuendeDoNotReduceTNCapacity(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// 1 vehicle (8-seat), 1 driver + 1 extra Betreuende, minGroupSize=6.
	// Old order (Betreuende before TN): effectiveCap = min(7, 8-2) = 6 — ok but tight.
	// Extra Betreuende before TN with a 7-seat vehicle: effectiveCap = min(7, 7-2) = 5 < 6.
	// New order (TN before extra Betreuende): effectiveCap = min(7, 7-1) = 6 — fine.
	tnRows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 0; i < 6; i++ {
		tnRows = append(tnRows, []string{"TN", "X", "14", "M", ""})
	}
	if err := database.InsertData(db, tnRows); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"Driver", "A", "ja"},
		{"Helper", "A", "nein"},
	}); err != nil {
		t.Fatalf("InsertBetreuende: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"Bus", "A", "", "Driver", "8"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	if _, err := services.CreateBalancedGroups(db, svcCfg(7, 6)); err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	if len(groups[0].Teilnehmende) != 6 {
		t.Errorf("expected 6 TN placed (non-driver Betreuende must not reduce TN capacity), got %d",
			len(groups[0].Teilnehmende))
	}
}

// TestCreateBalancedGroups_RebalanceBetreuendeTNRatio verifies that Phase 3d
// swaps a non-driver Betreuende from the group with the highest B:TN ratio
// into the group with the lowest ratio, in exchange for a Teilnehmende moving
// the other way. The total headcount per group must be unchanged (seat capacity
// is preserved automatically because each swap is 1-for-1).
func TestCreateBalancedGroups_RebalanceBetreuendeTNRatio(t *testing.T) {
	db := setupFullTestDB(t)
	defer teardownTestDB(t, db)

	// Setup:
	//   AlphaBus (OV A, 8 seats, driver "D1") → Group 1
	//   BetaBus  (OV B, 10 seats, driver "D2") → Group 2
	//
	// PreGroups force the initial TN split:
	//   "Bus_Alpha": 5 TN  → placed in Group 1 (same-OV preference wins tiebreak)
	//   "Bus_Beta":  8 TN  → placed in Group 2 (Group 1 has no room for 8)
	//
	// Helper (OV A, unlicensed) follows D1's OV → Phase 3 puts her in Group 1.
	// After Phase 3 (before rebalance):
	//   Group 1: 5 TN + 2 B (D1 + Helper) in 8 seats  → ratio 2/5 = 0.40
	//   Group 2: 8 TN + 1 B (D2)          in 10 seats → ratio 1/8 = 0.125
	//
	// Phase 3d must swap Helper (G1→G2) with 1 TN (G2→G1):
	//   Group 1: 6 TN + 1 B  → ratio 1/6 ≈ 0.167  (headcount 7, seats 8 ✓)
	//   Group 2: 7 TN + 2 B  → ratio 2/7 ≈ 0.286  (headcount 9, seats 10 ✓)
	tnRows := [][]string{{"Name", "Ortsverband", "Alter", "Geschlecht", "PreGroup"}}
	for i := 0; i < 5; i++ {
		tnRows = append(tnRows, []string{"TN_A", "A", "14", "M", "Bus_Alpha"})
	}
	for i := 0; i < 8; i++ {
		tnRows = append(tnRows, []string{"TN_B", "B", "14", "M", "Bus_Beta"})
	}
	if err := database.InsertData(db, tnRows); err != nil {
		t.Fatalf("InsertData: %v", err)
	}
	if err := database.InsertBetreuende(db, [][]string{
		{"Name", "Ortsverband", "Fahrerlaubnis"},
		{"D1", "A", "ja"},
		{"D2", "B", "ja"},
		{"Helper", "A", "nein"},
	}); err != nil {
		t.Fatalf("InsertBetreuende: %v", err)
	}
	if err := database.InsertFahrzeuge(db, [][]string{
		{"Bezeichnung", "Ortsverband", "Funkrufname", "FahrerName", "Sitzplaetze"},
		{"AlphaBus", "A", "", "D1", "8"},
		{"BetaBus", "B", "", "D2", "10"},
	}); err != nil {
		t.Fatalf("InsertFahrzeuge: %v", err)
	}

	_, err := services.CreateBalancedGroups(db, svcCfg(10, 0))
	if err != nil {
		t.Fatalf("CreateBalancedGroups: %v", err)
	}

	groups, err := database.GetGroupsForReport(db)
	if err != nil {
		t.Fatalf("GetGroupsForReport: %v", err)
	}
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	// Total TN must be preserved.
	totalTN := 0
	for _, g := range groups {
		totalTN += len(g.Teilnehmende)
	}
	if totalTN != 13 {
		t.Errorf("expected 13 TN total after rebalance, got %d", totalTN)
	}

	// No group must be overloaded (headcount > seats).
	seatsByVehicle := map[string]int{"AlphaBus": 8, "BetaBus": 10}
	for _, g := range groups {
		seats := 0
		for _, f := range g.Fahrzeuge {
			seats += seatsByVehicle[f.Bezeichnung]
		}
		total := len(g.Teilnehmende) + len(g.Betreuende)
		if total > seats {
			t.Errorf("group %d overloaded after rebalance: %d people in %d seats",
				g.GroupID, total, seats)
		}
	}

	// After one swap the Betreuende counts should be [1, 2] (one each became 1
	// and two — the group that started with 2 donated one).
	bCounts := make([]int, len(groups))
	for i, g := range groups {
		bCounts[i] = len(g.Betreuende)
	}
	has1, has2 := false, false
	for _, c := range bCounts {
		if c == 1 {
			has1 = true
		}
		if c == 2 {
			has2 = true
		}
	}
	if !has1 || !has2 {
		t.Errorf("expected one group with 1 Betreuende and one with 2, got %v", bCounts)
	}
}
