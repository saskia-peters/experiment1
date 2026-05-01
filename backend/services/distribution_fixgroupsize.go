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

// CarGroup is defined in models.CarGroup; this package uses it via models import.

// createGroupsFixGroupSize distributes participants into groups of
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
	numGroups := int(math.Round(float64(N) / float64(fixSize)))
	if numGroups < 1 {
		numGroups = 1
	}

	// ── Step 2: compute per-group capacities for even distribution ─────────────
	// extra groups receive (base+1) participants; the rest receive base.
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
	var warnings []string

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

	// ── Step 5: distribute Betreuende ─────────────────────────────────────────
	if len(betreuende) > 0 {
		w, err := distributeBetreuende(groups, betreuende)
		if err != nil {
			return "", err
		}
		if w != "" {
			warnings = append(warnings, w)
		}
	}

	// ── Step 6: save groups (TN + Betreuende) ─────────────────────────────────
	if err := database.SaveGroups(db, groups); err != nil {
		return "", fmt.Errorf("failed to save groups: %w", err)
	}
	if len(betreuende) > 0 {
		if err := database.SaveGroupBetreuende(db, groups); err != nil {
			return "", fmt.Errorf("failed to save group betreuende: %w", err)
		}
	}

	// ── Step 7: vehicle assignment ─────────────────────────────────────────────
	if len(fahrzeuge) == 0 {
		// No vehicles imported — nothing to assign.
	} else if strings.EqualFold(cfg.Verteilung.CarGroups, "ja") {
		carGroupWarn := assignCarGroups(groups, fahrzeuge, betreuende)
		if carGroupWarn != "" {
			warnings = append(warnings, carGroupWarn)
		}
		if err := database.SaveCarGroups(db, lastCarGroups); err != nil {
			return "", fmt.Errorf("failed to save cargroups: %w", err)
		}
	} else {
		// 1:1 vehicle assignment (cargroups = "nein").
		vehicleWarn := assignVehiclesOneToOne(groups, fahrzeuge, betreuende)
		if vehicleWarn != "" {
			warnings = append(warnings, vehicleWarn)
		}
		if err := database.SaveGroupFahrzeuge(db, groups); err != nil {
			return "", fmt.Errorf("failed to save group fahrzeuge: %w", err)
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

// assignVehiclesOneToOne assigns cars to groups using 1:1 matching:
// groups sorted by headcount descending, cars sorted by seat count descending.
// Returns a warning string for capacity issues or missing drivers.
func assignVehiclesOneToOne(groups []models.Group, fahrzeuge []models.Fahrzeug, betreuende []models.Betreuende) string {
	// Sort groups by total headcount descending.
	sorted := make([]int, len(groups))
	for i := range sorted {
		sorted[i] = i
	}
	sort.Slice(sorted, func(a, b int) bool {
		ha := len(groups[sorted[a]].Teilnehmende) + len(groups[sorted[a]].Betreuende)
		hb := len(groups[sorted[b]].Teilnehmende) + len(groups[sorted[b]].Betreuende)
		return ha > hb
	})

	// Sort cars by seat count descending.
	cars := make([]models.Fahrzeug, len(fahrzeuge))
	copy(cars, fahrzeuge)
	sort.Slice(cars, func(i, j int) bool {
		return cars[i].Sitzplaetze > cars[j].Sitzplaetze
	})

	// Build driver lookup.
	type driverKey struct{ name, ov string }
	driverMap := make(map[driverKey]*models.Betreuende)
	for i := range betreuende {
		if betreuende[i].Fahrerlaubnis {
			k := driverKey{
				name: strings.ToLower(strings.TrimSpace(betreuende[i].Name)),
				ov:   strings.ToLower(strings.TrimSpace(betreuende[i].Ortsverband)),
			}
			driverMap[k] = &betreuende[i]
		}
	}

	var warnParts []string
	limit := len(sorted)
	if len(cars) < limit {
		limit = len(cars)
	}

	for idx := 0; idx < limit; idx++ {
		gi := sorted[idx]
		car := cars[idx]
		groups[gi].Fahrzeuge = append(groups[gi].Fahrzeuge, car)

		// Resolve driver: check the car's named driver first; fall back to the
		// first licensed Betreuende in the group when the name is missing or
		// not found in the Betreuende list.
		carFahrzeugIdx := len(groups[gi].Fahrzeuge) - 1
		driverResolved := false
		if car.FahrerName != "" {
			k := driverKey{
				name: strings.ToLower(strings.TrimSpace(car.FahrerName)),
				ov:   strings.ToLower(strings.TrimSpace(car.Ortsverband)),
			}
			if _, found := driverMap[k]; found {
				driverResolved = true
			} else {
				warnParts = append(warnParts, fmt.Sprintf(
					"Fahrzeug %q: Fahrer %q (OV %q) nicht in der Betreuende-Liste gefunden",
					car.Bezeichnung, car.FahrerName, car.Ortsverband))
			}
		}
		if !driverResolved {
			for _, b := range groups[gi].Betreuende {
				if b.Fahrerlaubnis {
					groups[gi].Fahrzeuge[carFahrzeugIdx].FahrerName = b.Name
					break
				}
			}
		}
	}

	// Warn about groups without a car.
	for idx := limit; idx < len(sorted); idx++ {
		gi := sorted[idx]
		warnParts = append(warnParts, fmt.Sprintf(
			"Gruppe %d: kein Fahrzeug zugewiesen (mehr Gruppen als Fahrzeuge)",
			groups[gi].GroupID))
	}
	// Inform about unused cars.
	if len(cars) > len(sorted) {
		var unused []string
		for _, c := range cars[len(sorted):] {
			unused = append(unused, fmt.Sprintf("%q (OV %s, %d Plätze)", c.Bezeichnung, c.Ortsverband, c.Sitzplaetze))
		}
		warnParts = append(warnParts, fmt.Sprintf(
			"ℹ️ Nicht verwendete Fahrzeuge: %s", strings.Join(unused, "; ")))
	}

	return strings.Join(warnParts, "\n")
}

// ── CarGroups pool-assignment helpers ─────────────────────────────────────────

const (
	cgMaxGroupsPerPool = 3 // max participant groups that may share one pool
	cgMaxCarsPerPool   = 5 // max cars that may be assigned during Phase 1 solve
	cgMaxTolerance     = 3 // max acceptable empty seats per pool
)

// poolSolution is an intermediate result used during the pool-cover search.
// It records which participant groups and which (sorted) car indices form one pool.
type poolSolution struct {
	groupIdxs []int // indices into the groups slice passed to assignCarGroups
	carIdxs   []int // indices into the sorted cars slice
	headcount int   // total people across all groups in this pool
	seats     int   // total seats across all cars in this pool
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
// Anchoring to the lowest-index unassigned group prevents duplicate combinations
// across recursive calls.
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

// bestCarSubset uses 0/1-knapsack DP to find the minimum-car subset from
// available (indices into cars) whose seat total is in [headcount, headcount+maxTol]
// and whose count does not exceed maxCars.
//
// Primary criterion: fewest empty seats (= smallest valid seat total).
// Secondary criterion: fewest cars (already guaranteed by minimising count in DP).
//
// Returns (selected car indices into cars, total seats, found).
func bestCarSubset(cars []models.Fahrzeug, available []int, headcount, maxTol, maxCars int) ([]int, int, bool) {
	const inf = math.MaxInt32
	maxAvail := 0
	for _, ci := range available {
		maxAvail += cars[ci].Sitzplaetze
	}
	if headcount > maxAvail {
		return nil, 0, false
	}
	upper := headcount + maxTol
	if upper > maxAvail {
		upper = maxAvail
	}

	type dpState struct {
		count int
		mask  uint32 // bit ai set ⟺ available[ai] is included (ai = index within available)
	}
	dp := make([]dpState, upper+1)
	for i := range dp {
		dp[i].count = inf
	}
	dp[0] = dpState{count: 0}

	for ai, ci := range available {
		s := cars[ci].Sitzplaetze
		// Reverse iteration: 0/1 knapsack — each car used at most once.
		for total := upper; total >= s; total-- {
			prev := dp[total-s]
			if prev.count == inf {
				continue
			}
			newCount := prev.count + 1
			if newCount > maxCars {
				continue
			}
			if newCount < dp[total].count {
				dp[total] = dpState{
					count: newCount,
					mask:  prev.mask | (1 << uint(ai)),
				}
			}
		}
	}

	// Scan low→high: first valid entry has the fewest empty seats.
	bestTotal := -1
	for t := headcount; t <= upper; t++ {
		if dp[t].count < inf {
			bestTotal = t
			break
		}
	}
	if bestTotal < 0 {
		return nil, 0, false
	}

	mask := dp[bestTotal].mask
	selected := make([]int, 0, dp[bestTotal].count)
	for ai, ci := range available {
		if mask&(1<<uint(ai)) != 0 {
			selected = append(selected, ci)
		}
	}
	return selected, bestTotal, true
}

// solvePoolCover recursively assigns all participant groups to car pools using
// depth-first search with backtracking.
//
// Strategy: always start from the lowest-index unassigned group (the "anchor"),
// generate all valid pool candidates that include it, sort by quality, and try
// each one before backtracking. This guarantees each combination is explored
// exactly once.
func solvePoolCover(
	groups []models.Group,
	cars []models.Fahrzeug,
	usedGroups, usedCars []bool,
	current []poolSolution,
) ([]poolSolution, bool) {
	// Find the first unassigned group (anchor).
	anchor := -1
	for i, used := range usedGroups {
		if !used {
			anchor = i
			break
		}
	}
	if anchor < 0 {
		return current, true // all groups assigned
	}

	// Collect available groups and cars.
	availGroups := make([]int, 0, len(groups))
	for i, used := range usedGroups {
		if !used {
			availGroups = append(availGroups, i)
		}
	}
	availCars := make([]int, 0, len(cars))
	for i, used := range usedCars {
		if !used {
			availCars = append(availCars, i)
		}
	}

	// Generate candidates: for each group-combo size 1..3 containing anchor,
	// find the best fitting car subset.
	var cands []poolSolution
	for sz := 1; sz <= min(cgMaxGroupsPerPool, len(availGroups)); sz++ {
		for _, groupCombo := range combineGroups(availGroups, anchor, sz) {
			headcount := 0
			for _, gi := range groupCombo {
				headcount += len(groups[gi].Teilnehmende) + len(groups[gi].Betreuende)
			}
			carIdxs, seats, found := bestCarSubset(cars, availCars, headcount, cgMaxTolerance, cgMaxCarsPerPool)
			if !found {
				continue
			}
			cands = append(cands, poolSolution{
				groupIdxs: groupCombo,
				carIdxs:   carIdxs,
				headcount: headcount,
				seats:     seats,
			})
		}
	}

	if len(cands) == 0 {
		return nil, false
	}

	// Sort: fewest empty seats → fewest cars → most groups (prefer larger pools).
	sort.Slice(cands, func(i, j int) bool {
		ei := cands[i].seats - cands[i].headcount
		ej := cands[j].seats - cands[j].headcount
		if ei != ej {
			return ei < ej
		}
		if len(cands[i].carIdxs) != len(cands[j].carIdxs) {
			return len(cands[i].carIdxs) < len(cands[j].carIdxs)
		}
		return len(cands[i].groupIdxs) > len(cands[j].groupIdxs)
	})

	// Try each candidate; backtrack on failure.
	for _, c := range cands {
		for _, gi := range c.groupIdxs {
			usedGroups[gi] = true
		}
		for _, ci := range c.carIdxs {
			usedCars[ci] = true
		}

		// Create a fresh slice so backtracking at this level cannot corrupt it.
		newCurrent := make([]poolSolution, len(current)+1)
		copy(newCurrent, current)
		newCurrent[len(current)] = c

		if result, ok := solvePoolCover(groups, cars, usedGroups, usedCars, newCurrent); ok {
			return result, true
		}

		// Backtrack.
		for _, gi := range c.groupIdxs {
			usedGroups[gi] = false
		}
		for _, ci := range c.carIdxs {
			usedCars[ci] = false
		}
	}
	return nil, false
}

// assignCarGroups assigns participant groups to car pools, minimising empty
// seats per pool (tolerance 0..3 empty seats, max 3 groups and 5 cars per pool).
//
//   Phase 1 – solve: depth-first search finds the optimal (group, car-subset)
//              assignment. Each pool has 1–3 groups and 1–5 cars. Empty seats
//              per pool are minimised; backtracking resolves dead ends.
//   Phase 2 – remaining cars: any car not assigned in Phase 1 is added to the
//              pool with the least spare capacity (tightest pool first).
//   Phase 3 – driver resolution: xlsx-provided FahrerName is kept as-is (cars
//              may have specialist drivers such as truck licence holders that are
//              not in the Betreuende list). Only cars without a pre-assigned
//              driver receive a fallback from the licensed Betreuende in the pool.
//
// Returns a warning string for capacity or driver issues.
func assignCarGroups(groups []models.Group, fahrzeuge []models.Fahrzeug, betreuende []models.Betreuende) string {
	if len(groups) == 0 {
		return ""
	}

	// Sort cars by seat count descending so the DP prefers large cars first.
	cars := make([]models.Fahrzeug, len(fahrzeuge))
	copy(cars, fahrzeuge)
	sort.Slice(cars, func(i, j int) bool {
		return cars[i].Sitzplaetze > cars[j].Sitzplaetze
	})

	var warnParts []string

	// ── Phase 1: solve pool assignment ────────────────────────────────────────
	usedGroups := make([]bool, len(groups))
	usedCars := make([]bool, len(cars))
	pools, ok := solvePoolCover(groups, cars, usedGroups, usedCars, nil)

	if !ok {
		// Fallback: one group per pool, greedy minimum-car coverage.
		warnParts = append(warnParts,
			"⚠️ Keine optimale Fahrgemeinschaftsaufteilung gefunden – Fallback: eine Gruppe pro Pool")
		pools = make([]poolSolution, len(groups))
		carIdx := 0
		for i := range groups {
			headcount := len(groups[i].Teilnehmende) + len(groups[i].Betreuende)
			var carIdxs []int
			seats := 0
			for carIdx < len(cars) && seats < headcount {
				usedCars[carIdx] = true
				carIdxs = append(carIdxs, carIdx)
				seats += cars[carIdx].Sitzplaetze
				carIdx++
			}
			pools[i] = poolSolution{
				groupIdxs: []int{i},
				carIdxs:   carIdxs,
				headcount: headcount,
				seats:     seats,
			}
		}
	}

	// Build CarGroup objects from the pool solutions.
	carGroups := make([]*models.CarGroup, len(pools))
	for i, p := range pools {
		cg := &models.CarGroup{ID: i + 1}
		for _, gi := range p.groupIdxs {
			cg.Groups = append(cg.Groups, groups[gi])
		}
		for _, ci := range p.carIdxs {
			cg.Cars = append(cg.Cars, cars[ci])
		}
		carGroups[i] = cg
	}

	// Warn about pools where seats still fall short of people.
	for _, cg := range carGroups {
		people := 0
		for _, g := range cg.Groups {
			people += len(g.Teilnehmende) + len(g.Betreuende)
		}
		seats := 0
		for _, c := range cg.Cars {
			seats += c.Sitzplaetze
		}
		if seats < people {
			warnParts = append(warnParts, fmt.Sprintf(
				"⚠️ Fahrzeugpool %d: %d Personen, aber nur %d Sitzplätze verfügbar",
				cg.ID, people, seats))
		}
	}

	// ── Phase 2: distribute remaining cars (all must be used) ─────────────────
	// Give each leftover car to the pool with the least spare capacity.
	for ci, used := range usedCars {
		if used {
			continue
		}
		bestIdx := 0
		bestSpare := math.MaxInt32
		for pi, cg := range carGroups {
			people := 0
			for _, g := range cg.Groups {
				people += len(g.Teilnehmende) + len(g.Betreuende)
			}
			cgSeats := 0
			for _, c := range cg.Cars {
				cgSeats += c.Sitzplaetze
			}
			if spare := cgSeats - people; spare < bestSpare {
				bestSpare = spare
				bestIdx = pi
			}
		}
		carGroups[bestIdx].Cars = append(carGroups[bestIdx].Cars, cars[ci])
	}

	// ── Phase 3: resolve car drivers ──────────────────────────────────────────
	// Cars that already have a FahrerName from the xlsx are kept as-is; this
	// preserves specialist drivers (e.g. truck licence holders) that may not
	// appear in the Betreuende list.  Only cars without a pre-assigned driver
	// receive a fallback from the licensed Betreuende in the pool.

	// usedDriverNames tracks lower-cased names already driving to avoid
	// assigning the same fallback driver to two cars.
	usedDriverNames := make(map[string]bool)

	// Seed the set with names already present in the xlsx.
	for _, cg := range carGroups {
		for _, car := range cg.Cars {
			if car.FahrerName != "" {
				usedDriverNames[strings.ToLower(strings.TrimSpace(car.FahrerName))] = true
			}
		}
	}

	for _, cg := range carGroups {
		for ci := range cg.Cars {
			car := &cg.Cars[ci]
			if car.FahrerName != "" {
				// Driver was provided in the xlsx — trust it.
				continue
			}
			// No driver set — pick the first available licensed Betreuende in this pool.
			for _, g := range cg.Groups {
				for _, b := range g.Betreuende {
					if !b.Fahrerlaubnis {
						continue
					}
					normalName := strings.ToLower(strings.TrimSpace(b.Name))
					if usedDriverNames[normalName] {
						continue
					}
					car.FahrerName = b.Name
					usedDriverNames[normalName] = true
					goto nextCar
				}
			}
			// No available driver found in this pool.
			warnParts = append(warnParts, fmt.Sprintf(
				"Fahrzeug %q: kein freier Fahrer im Pool verfügbar", car.Bezeichnung))
		nextCar:
		}
	}

	lastCarGroups = carGroups
	return strings.Join(warnParts, "\n")
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
