package services

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"THW-JugendOlympiade/backend/config"
	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/models"
)

// bGroup is the internal representation of a betreuende-group formed before
// pairing with a participant group.  It holds all betreuende assigned to it and
// all cars whose named drivers are in this group.
type bGroup struct {
	betreuende []models.Betreuende
	fahrzeuge  []models.Fahrzeug
}

// totalSeats returns the combined seat count of all cars in a bGroup.
func (bg *bGroup) totalSeats() int {
	s := 0
	for _, f := range bg.fahrzeuge {
		s += f.Sitzplaetze
	}
	return s
}

// validateAndMapDrivers validates the Fahrzeuge list and returns a map from
// Fahrzeug.ID to the matching Betreuende (driver).
//
// Hard errors (returned as error):
//   - Any Fahrzeug has an empty FahrerName
//   - Any FahrerName does not match any Betreuende (case-insensitive, trimmed)
//   - The same Betreuende is named as driver for two or more cars
//   - The number of distinct drivers is less than numGroups
//
// Soft warnings (returned in []string):
//   - Fahrzeug.Ortsverband differs from the matched Betreuende.Ortsverband
//   - numDrivers > numGroups (extra drivers; handled by formBetreuendeGroups)
func validateAndMapDrivers(
	fahrzeuge []models.Fahrzeug,
	betreuende []models.Betreuende,
	numGroups int,
) (driverByCarID map[int]*models.Betreuende, warnings []string, err error) {
	// Build name → Betreuende index (case-insensitive, trimmed).
	betByName := make(map[string]int, len(betreuende))
	for i, b := range betreuende {
		key := strings.ToLower(strings.TrimSpace(b.Name))
		if _, exists := betByName[key]; !exists {
			betByName[key] = i
		}
	}

	driverByCarID = make(map[int]*models.Betreuende, len(fahrzeuge))
	claimedByName := make(map[string]int) // driver name key → car ID that claimed it

	for _, car := range fahrzeuge {
		if car.FahrerName == "" {
			return nil, nil, fmt.Errorf(
				"Fahrzeug %q (OV %s) hat keinen Fahrer eingetragen – "+
					"alle Fahrzeuge müssen einen Fahrer haben",
				car.Bezeichnung, car.Ortsverband)
		}
		nameKey := strings.ToLower(strings.TrimSpace(car.FahrerName))
		bi, found := betByName[nameKey]
		if !found {
			return nil, nil, fmt.Errorf(
				"Fahrzeug %q: Fahrer %q nicht in der Betreuenden-Liste gefunden – "+
					"bitte Namen prüfen oder Betreuende-Tabelle ergänzen",
				car.Bezeichnung, car.FahrerName)
		}
		if prevCarID, dup := claimedByName[nameKey]; dup {
			return nil, nil, fmt.Errorf(
				"Betreuende %q ist als Fahrer für mehrere Fahrzeuge eingetragen "+
					"(Fahrzeug-ID %d und %d) – jede Person kann nur ein Fahrzeug fahren",
				car.FahrerName, prevCarID, car.ID)
		}
		claimedByName[nameKey] = car.ID
		b := &betreuende[bi]
		driverByCarID[car.ID] = b

		// Soft warning: OV mismatch.
		betOV := strings.ToLower(strings.TrimSpace(b.Ortsverband))
		carOV := strings.ToLower(strings.TrimSpace(car.Ortsverband))
		if betOV != carOV {
			warnings = append(warnings, fmt.Sprintf(
				"Fahrzeug %q: Fahrer %q gefunden, aber OV unterscheidet sich "+
					"(Fahrzeuge: %q, Betreuende: %q) — wird trotzdem zugeordnet",
				car.Bezeichnung, car.FahrerName, car.Ortsverband, b.Ortsverband))
		}
	}

	numDrivers := len(driverByCarID)
	if numDrivers < numGroups {
		return nil, nil, fmt.Errorf(
			"zu wenig Fahrer: %d Fahrer (Fahrzeuge) für %d Gruppen – "+
				"bitte mindestens %d Fahrzeuge mit eingetragenen Fahrern importieren",
			numDrivers, numGroups, numGroups)
	}
	if numDrivers > numGroups {
		warnings = append(warnings, fmt.Sprintf(
			"ℹ️ %d Fahrer für %d Gruppen – überzählige Fahrer werden auf die Gruppen verteilt",
			numDrivers, numGroups))
	}
	return driverByCarID, warnings, nil
}

// formBetreuendeGroups creates exactly numGroups betreuende-groups, each with
// ≥ 1 driver (and their car) and ≥ 2 betreuende total.
//
//   - Phase 2a: assign the first numGroups drivers (OV round-robin order) as
//     anchors, 1 per B-group; their car(s) follow.
//   - Phase 2b: assign extra drivers to B-groups preferring same-OV clusters.
//   - Phase 2c: ensure every B-group with only 1 betreuende gets a non-driver,
//     preferring same-OV non-drivers.
//   - Phase 2d: distribute all remaining non-drivers evenly, preferring same-OV.
func formBetreuendeGroups(
	numGroups int,
	betreuende []models.Betreuende,
	fahrzeuge []models.Fahrzeug,
	driverByCarID map[int]*models.Betreuende,
) ([]bGroup, string, error) {
	// Separate drivers from non-drivers.
	driverSet := make(map[int]bool, len(driverByCarID)) // Betreuende.ID → is driver
	for _, b := range driverByCarID {
		driverSet[b.ID] = true
	}

	var drivers, nonDrivers []models.Betreuende
	for _, b := range betreuende {
		if driverSet[b.ID] {
			drivers = append(drivers, b)
		} else {
			nonDrivers = append(nonDrivers, b)
		}
	}

	// Build car lookup: driver Betreuende.ID → list of cars driven.
	carsByDriverID := make(map[int][]models.Fahrzeug, len(driverByCarID))
	for _, car := range fahrzeuge {
		drv, ok := driverByCarID[car.ID]
		if !ok {
			continue
		}
		carsByDriverID[drv.ID] = append(carsByDriverID[drv.ID], car)
	}

	// ── Phase 2a: assign first numGroups drivers (OV round-robin) as anchors ──
	ordered := ovRoundRobinOrder(drivers)
	bgroups := make([]bGroup, numGroups)
	for i := 0; i < numGroups; i++ {
		drv := ordered[i]
		bgroups[i].betreuende = []models.Betreuende{drv}
		bgroups[i].fahrzeuge = carsByDriverID[drv.ID]
	}

	// ── Phase 2b: assign extra drivers to B-groups (same-OV preference) ───────
	for _, drv := range ordered[numGroups:] {
		bestIdx := 0
		bestOVCount := -1
		bestBetCount := math.MaxInt32
		for i, bg := range bgroups {
			// Count how many drivers in this B-group share drv's OV.
			ovCount := 0
			for _, b := range bg.betreuende {
				if b.Ortsverband == drv.Ortsverband {
					ovCount++
				}
			}
			betCount := len(bg.betreuende)
			if ovCount > bestOVCount || (ovCount == bestOVCount && betCount < bestBetCount) {
				bestOVCount = ovCount
				bestBetCount = betCount
				bestIdx = i
			}
		}
		bgroups[bestIdx].betreuende = append(bgroups[bestIdx].betreuende, drv)
		bgroups[bestIdx].fahrzeuge = append(bgroups[bestIdx].fahrzeuge, carsByDriverID[drv.ID]...)
	}

	// Sort non-drivers deterministically: OV then name.
	sort.Slice(nonDrivers, func(i, j int) bool {
		if nonDrivers[i].Ortsverband != nonDrivers[j].Ortsverband {
			return nonDrivers[i].Ortsverband < nonDrivers[j].Ortsverband
		}
		return nonDrivers[i].Name < nonDrivers[j].Name
	})

	// ── Phase 2c: fill B-groups that have only 1 betreuende to meet min-2 ──────
	// For each single-member B-group, pick same-OV non-driver first.
	used := make([]bool, len(nonDrivers))
	for i := range bgroups {
		if len(bgroups[i].betreuende) >= 2 {
			continue
		}
		driverOV := bgroups[i].betreuende[0].Ortsverband
		picked := -1
		// Pass 1: same-OV non-driver.
		for k, nd := range nonDrivers {
			if !used[k] && nd.Ortsverband == driverOV {
				picked = k
				break
			}
		}
		// Pass 2: any non-driver.
		if picked < 0 {
			for k := range nonDrivers {
				if !used[k] {
					picked = k
					break
				}
			}
		}
		if picked < 0 {
			// No non-driver available — check if another B-group can donate a driver.
			// Find a donor B-group with ≥ 3 betreuende (so it can donate and still have ≥ 2).
			donorIdx := -1
			for j, bg := range bgroups {
				if j != i && len(bg.betreuende) >= 3 {
					donorIdx = j
					break
				}
			}
			if donorIdx < 0 {
				return nil, "", fmt.Errorf(
					"zu wenig Betreuende: Betreuendengruppe %d hat nur 1 Betreuende und es gibt "+
						"keine Nicht-Fahrer mehr – bitte mindestens %d Betreuende importieren "+
						"(2 pro Gruppe = mindestens %d)",
					i+1, 2*numGroups, 2*numGroups)
			}
			// Move a non-driver from donor if possible, else any last betreuende.
			donor := &bgroups[donorIdx]
			moveIdx := -1
			for k, b := range donor.betreuende {
				if !driverSet[b.ID] {
					moveIdx = k
					break
				}
			}
			if moveIdx < 0 {
				moveIdx = len(donor.betreuende) - 1 // move last as fallback
			}
			moved := donor.betreuende[moveIdx]
			donor.betreuende = append(donor.betreuende[:moveIdx], donor.betreuende[moveIdx+1:]...)
			bgroups[i].betreuende = append(bgroups[i].betreuende, moved)
			continue
		}
		bgroups[i].betreuende = append(bgroups[i].betreuende, nonDrivers[picked])
		used[picked] = true
	}

	// ── Phase 2d: distribute remaining non-drivers evenly (same-OV preferred) ─
	for k, nd := range nonDrivers {
		if used[k] {
			continue
		}
		bestIdx := 0
		bestOVCount := -1
		bestBetCount := math.MaxInt32
		for i, bg := range bgroups {
			ovCount := 0
			for _, b := range bg.betreuende {
				if b.Ortsverband == nd.Ortsverband {
					ovCount++
				}
			}
			betCount := len(bg.betreuende)
			if ovCount > bestOVCount || (ovCount == bestOVCount && betCount < bestBetCount) {
				bestOVCount = ovCount
				bestBetCount = betCount
				bestIdx = i
			}
		}
		bgroups[bestIdx].betreuende = append(bgroups[bestIdx].betreuende, nd)
	}

	// Build warnings.
	var warnParts []string
	for i, bg := range bgroups {
		if len(bg.betreuende) < 2 {
			warnParts = append(warnParts, fmt.Sprintf(
				"⚠️ Betreuendengruppe %d hat nur %d Betreuende (Minimum: 2)",
				i+1, len(bg.betreuende)))
		}
	}
	return bgroups, strings.Join(warnParts, "\n"), nil
}

// pairAndAssignUnits sorts P-groups by TN count descending and B-groups by
// total car seats descending, then pairs them by index (most TN ↔ most seats).
// Betreuende and Fahrzeuge from each B-group are written into the paired Group.
func pairAndAssignUnits(groups []models.Group, bgroups []bGroup) {
	// Sort P-group indices by TN count descending.
	pgIdx := make([]int, len(groups))
	for i := range pgIdx {
		pgIdx[i] = i
	}
	sort.Slice(pgIdx, func(a, b int) bool {
		return len(groups[pgIdx[a]].Teilnehmende) > len(groups[pgIdx[b]].Teilnehmende)
	})

	// Sort B-group indices by total car seats descending.
	bgIdx := make([]int, len(bgroups))
	for i := range bgIdx {
		bgIdx[i] = i
	}
	sort.Slice(bgIdx, func(a, b int) bool {
		return bgroups[bgIdx[a]].totalSeats() > bgroups[bgIdx[b]].totalSeats()
	})

	// Pair by index: P-group pgIdx[i] gets B-group bgIdx[i].
	for i := 0; i < len(pgIdx) && i < len(bgIdx); i++ {
		gi := pgIdx[i]
		bi := bgIdx[i]
		groups[gi].Betreuende = bgroups[bi].betreuende
		groups[gi].Fahrzeuge = bgroups[bi].fahrzeuge
	}
}

// solveUnitCarpools groups paired units (each Group with its own Betreuende and
// Fahrzeuge already assigned) into carpools of 1–3 units. The combined seat
// count of all cars in a pool must be ≥ the combined headcount (TN + Betreuende)
// of all groups in the pool. The solver minimises empty seats; larger pools are
// preferred as a tiebreaker.
//
// Uses depth-first search with backtracking, anchored on the lowest-index
// unassigned group so each combination is explored exactly once.
func solveUnitCarpools(groups []models.Group) []*models.CarGroup {
	n := len(groups)
	if n == 0 {
		return nil
	}

	// Precompute per-group headcount and seat count.
	headcounts := make([]int, n)
	seats := make([]int, n)
	for i, g := range groups {
		headcounts[i] = len(g.Teilnehmende) + len(g.Betreuende)
		for _, f := range g.Fahrzeuge {
			seats[i] += f.Sitzplaetze
		}
	}

	type poolResult struct {
		groupIdxs []int
	}

	var solve func(used []bool, current []poolResult) ([]poolResult, bool)
	solve = func(used []bool, current []poolResult) ([]poolResult, bool) {
		// Find anchor (first unassigned group).
		anchor := -1
		for i, u := range used {
			if !u {
				anchor = i
				break
			}
		}
		if anchor < 0 {
			return current, true // all assigned
		}

		// Build list of available group indices.
		avail := make([]int, 0, n)
		for i, u := range used {
			if !u {
				avail = append(avail, i)
			}
		}

		// Generate all pool candidates of size 1..3 containing anchor.
		type cand struct {
			idxs      []int
			empty     int // seats - headcount (lower is better)
			numGroups int
		}
		var cands []cand
		maxSz := 3
		if len(avail) < maxSz {
			maxSz = len(avail)
		}
		for sz := 1; sz <= maxSz; sz++ {
			for _, combo := range combineGroups(avail, anchor, sz) {
				totalHC := 0
				totalSeats := 0
				for _, gi := range combo {
					totalHC += headcounts[gi]
					totalSeats += seats[gi]
				}
				if totalSeats < totalHC {
					continue // not enough seats — skip
				}
				cands = append(cands, cand{
					idxs:      combo,
					empty:     totalSeats - totalHC,
					numGroups: sz,
				})
			}
		}

		if len(cands) == 0 {
			return nil, false
		}

		// Sort: fewest empty seats first; larger pool (more groups) as tiebreaker.
		sort.Slice(cands, func(i, j int) bool {
			if cands[i].empty != cands[j].empty {
				return cands[i].empty < cands[j].empty
			}
			return cands[i].numGroups > cands[j].numGroups
		})

		for _, c := range cands {
			for _, gi := range c.idxs {
				used[gi] = true
			}
			next := make([]poolResult, len(current)+1)
			copy(next, current)
			next[len(current)] = poolResult{groupIdxs: c.idxs}
			if result, ok := solve(used, next); ok {
				return result, true
			}
			for _, gi := range c.idxs {
				used[gi] = false
			}
		}
		return nil, false
	}

	used := make([]bool, n)
	results, ok := solve(used, nil)
	if !ok {
		// Fallback: each group is its own pool.
		results = make([]poolResult, n)
		for i := range results {
			results[i] = poolResult{groupIdxs: []int{i}}
		}
	}

	carGroups := make([]*models.CarGroup, len(results))
	for i, r := range results {
		cg := &models.CarGroup{ID: i + 1}
		for _, gi := range r.groupIdxs {
			cg.Groups = append(cg.Groups, groups[gi])
			cg.Cars = append(cg.Cars, groups[gi].Fahrzeuge...)
		}
		carGroups[i] = cg
	}
	return carGroups
}

// combineFrom returns all k-element subsets of items.
func combineFrom(items []int, k int) [][]int {
	if k == 0 {
		return [][]int{{}}
	}
	if k > len(items) {
		return nil
	}
	var result [][]int
	for i, x := range items {
		for _, rest := range combineFrom(items[i+1:], k-1) {
			combo := make([]int, 0, k)
			combo = append(combo, x)
			combo = append(combo, rest...)
			result = append(result, combo)
		}
	}
	return result
}

// combineGroups returns all subsets of size `size` from `items` that include `anchor`.
func combineGroups(items []int, anchor, size int) [][]int {
	if size == 1 {
		return [][]int{{anchor}}
	}
	rest := make([]int, 0, len(items)-1)
	for _, x := range items {
		if x != anchor {
			rest = append(rest, x)
		}
	}
	subs := combineFrom(rest, size-1)
	out := make([][]int, 0, len(subs))
	for _, sub := range subs {
		combo := make([]int, 0, size)
		combo = append(combo, anchor)
		combo = append(combo, sub...)
		out = append(out, combo)
	}
	return out
}

// approximately cfg.Verteilung.FixGroupSize, then optionally assigns vehicles
// via the CarGroups algorithm (cargroups = "ja") or 1:1 assignment (cargroups = "nein").
func createGroupsFixGroupSize(db *sql.DB, cfg config.Config) (string, error) {
	fixSize := cfg.Verteilung.FixGroupSize
	if fixSize < 1 {
		fixSize = 8
	}

	teilnehmende, err := database.GetAllTeilnehmende(db)
	if err != nil {
		return "", fmt.Errorf("failed to read teilnehmende: %w", err)
	}
	if len(teilnehmende) == 0 {
		return "", nil
	}

	if err := validatePreGroups(teilnehmende, fixSize); err != nil {
		return "", err
	}

	betreuende, err := database.GetAllBetreuende(db)
	if err != nil {
		return "", fmt.Errorf("failed to read betreuende: %w", err)
	}

	fahrzeuge, err := database.GetAllFahrzeuge(db)
	if err != nil {
		return "", fmt.Errorf("failed to read fahrzeuge: %w", err)
	}

	N := len(teilnehmende)

	// ── Step 1: compute number of groups ───────────────────────────────────────
	// Use ceiling division so that no group ever exceeds fixSize.
	// math.Round can round down, leaving too few groups and pushing one group
	// over the hard maximum (e.g. 105/8 → round(13.125)=13 → one group of 9).
	numGroups := (N + fixSize - 1) / fixSize
	if numGroups < 1 {
		numGroups = 1
	}

	// ── Step 2: compute per-group capacities for even distribution ─────────────
	// extra groups receive (base+1) participants; the rest receive base.
	// With ceiling division base ≤ fixSize and base+1 ≤ fixSize always holds.
	var warnings []string
	base := N / numGroups
	extra := N % numGroups

	groupCaps := make([]int, numGroups)
	for i := range groupCaps {
		if i < extra {
			groupCaps[i] = base + 1
		} else {
			groupCaps[i] = base
		}
	}

	// Warn when any group would be smaller than fixSize-2 (groups too sparse).
	minCap := base // base ≤ base+1, so base is the minimum capacity
	if minCap < fixSize-2 {
		warnings = append(warnings, fmt.Sprintf(
			"⚠️ Mit %d Teilnehmenden und fixgroupsize=%d entstehen %d Gruppen – "+
				"die kleinsten Gruppen haben nur %d Mitglieder (Minimum wäre fixgroupsize−2 = %d). "+
				"Bitte fixgroupsize anpassen.",
			N, fixSize, numGroups, minCap, fixSize-2))
	}

	// ── Step 3: initialise groups ──────────────────────────────────────────────
	groups := make([]models.Group, numGroups)
	for i := range groups {
		groups[i] = models.Group{
			GroupID:      i + 1,
			Teilnehmende: make([]models.Teilnehmende, 0, groupCaps[i]),
			Ortsverbands: make(map[string]int),
			Geschlechts:  make(map[string]int),
		}
	}

	// ── Step 4: distribute participants ───────────────────────────────────────
	preGroupMap, unassigned := separateByPreGroup(teilnehmende)

	// Place PreGroups first (best-fit by OV affinity, then fewest TN).
	preGroupKeys := make([]string, 0, len(preGroupMap))
	for k := range preGroupMap {
		preGroupKeys = append(preGroupKeys, k)
	}
	sort.Strings(preGroupKeys)

	for _, key := range preGroupKeys {
		members := preGroupMap[key]
		bestIdx := -1
		bestScore := math.MaxFloat64
		for i, g := range groups {
			remaining := groupCaps[i] - len(g.Teilnehmende)
			if remaining < len(members) {
				continue
			}
			ovBonus := 0.0
			for _, m := range members {
				ovBonus += float64(g.Ortsverbands[m.Ortsverband])
			}
			score := -ovBonus*2.0 + float64(len(g.Teilnehmende))*0.5
			if score < bestScore {
				bestScore = score
				bestIdx = i
			}
		}
		if bestIdx < 0 {
			return "", fmt.Errorf(
				"Vorgruppe %q (%d Mitglieder) passt in keine verfügbare Gruppe – "+
					"bitte fixgroupsize erhöhen oder die Vorgruppe aufteilen",
				key, len(members))
		}
		for _, t := range members {
			addTeilnehmendeToGroup(&groups[bestIdx], t)
		}
	}

	// Sort remaining TN for better diversity.
	sort.Slice(unassigned, func(i, j int) bool {
		if unassigned[i].Ortsverband != unassigned[j].Ortsverband {
			return unassigned[i].Ortsverband < unassigned[j].Ortsverband
		}
		if unassigned[i].Geschlecht != unassigned[j].Geschlecht {
			return unassigned[i].Geschlecht < unassigned[j].Geschlecht
		}
		return unassigned[i].Alter < unassigned[j].Alter
	})

	// Distribute remaining TN using diversity score, respecting per-group caps.
	for _, tn := range unassigned {
		bestIdx := findBestGroupFixSize(groups, groupCaps, tn)
		addTeilnehmendeToGroup(&groups[bestIdx], tn)
	}

	// ── Step 5: assign betreuende and vehicles ───────────────────────────────
	if len(fahrzeuge) == 0 {
		// No vehicles imported — distribute betreuende freely across groups
		// (only if there are betreuende to distribute).
		if len(betreuende) > 0 {
			w, err := distributeBetreuende(groups, betreuende)
			if err != nil {
				return "", err
			}
			if w != "" {
				warnings = append(warnings, w)
			}
		}
	} else {
		// Vehicles present — validate drivers and form betreuende-groups first.
		// Hard errors: empty FahrerName, name not found, duplicate driver, too few drivers.
		driverByCarID, driverWarns, driverErr := validateAndMapDrivers(fahrzeuge, betreuende, numGroups)
		warnings = append(warnings, driverWarns...)
		if driverErr != nil {
			return "", driverErr
		}

		// Form betreuende-groups (min 2 per group, each anchored to a driver).
		bgroups, bgWarn, bgErr := formBetreuendeGroups(numGroups, betreuende, fahrzeuge, driverByCarID)
		if bgWarn != "" {
			warnings = append(warnings, bgWarn)
		}
		if bgErr != nil {
			return "", bgErr
		}

		if strings.EqualFold(cfg.Verteilung.CarGroups, "ja") {
			// Pair B-groups to P-groups (most seats ↔ most TN).
			pairAndAssignUnits(groups, bgroups)

			// Form carpools from the paired units.
			carGroupList := solveUnitCarpools(groups)

			// Capacity warning for any pool that is over-full.
			for _, cg := range carGroupList {
				people := 0
				totalSeats := 0
				for _, g := range cg.Groups {
					people += len(g.Teilnehmende) + len(g.Betreuende)
				}
				for _, c := range cg.Cars {
					totalSeats += c.Sitzplaetze
				}
				if totalSeats < people {
					warnings = append(warnings, fmt.Sprintf(
						"⚠️ Fahrzeugpool %d: %d Personen, aber nur %d Sitzplätze — "+
							"bitte größere Fahrzeuge importieren oder Gruppengröße reduzieren",
						cg.ID, people, totalSeats))
				}
			}

			lastCarGroups = carGroupList
		} else {
			// 1:1 mode (cargroups = "nein"): pair B-groups to P-groups but skip
			// carpool formation.
			pairAndAssignUnits(groups, bgroups)
		}
	}

	// ── Step 6: save TN + vehicle assignments ────────────────────────────────
	if err := database.SaveGroups(db, groups); err != nil {
		return "", fmt.Errorf("failed to save groups: %w", err)
	}
	if len(fahrzeuge) > 0 {
		if strings.EqualFold(cfg.Verteilung.CarGroups, "ja") {
			if err := database.SaveCarGroups(db, lastCarGroups); err != nil {
				return "", fmt.Errorf("failed to save cargroups: %w", err)
			}
		} else {
			if err := database.SaveGroupFahrzeuge(db, groups); err != nil {
				return "", fmt.Errorf("failed to save group fahrzeuge: %w", err)
			}
		}
	}

	// ── Step 7: save Betreuende ────────────────────────────────────────────────
	if len(betreuende) > 0 {
		if err := database.SaveGroupBetreuende(db, groups); err != nil {
			return "", fmt.Errorf("failed to save group betreuende: %w", err)
		}
	}

	fmt.Printf("FixGroupSize: created %d groups (fixgroupsize=%d, N=%d)\n", numGroups, fixSize, N)
	for i, g := range groups {
		fmt.Printf("  Group %d: %d TN\n", i+1, len(g.Teilnehmende))
	}

	return strings.Join(warnings, "\n"), nil
}

// findBestGroupFixSize finds the best group for a participant, respecting the
// per-group capacity caps (groupCaps[i]).
func findBestGroupFixSize(groups []models.Group, groupCaps []int, tn models.Teilnehmende) int {
	bestIdx := 0
	bestScore := math.MaxFloat64

	for i, group := range groups {
		if len(group.Teilnehmende) >= groupCaps[i] {
			continue
		}
		score := calculateDiversityScore(group, tn)
		sizeBonus := float64(len(group.Teilnehmende)) * 0.5
		if score+sizeBonus < bestScore {
			bestScore = score + sizeBonus
			bestIdx = i
		}
	}
	return bestIdx
}

// GetLastCarGroups returns the CarGroups computed by the most recent
// FixGroupSize distribution run with cargroups="ja".
// Returns nil if no such run has been performed yet.
func GetLastCarGroups() []*models.CarGroup {
	return lastCarGroups
}

// SetLastCarGroups replaces the in-memory CarGroups. Used by the startup
// restore path to reload persisted pool assignments after a backup/restore.
func SetLastCarGroups(cgs []*models.CarGroup) {
	lastCarGroups = cgs
}

// lastCarGroups holds the most recently computed CarGroups for PDF generation.
// This is populated by createGroupsFixGroupSize when cargroups="ja" and read
// by the CarGroup PDF generator via GetLastCarGroups.
var lastCarGroups []*models.CarGroup
