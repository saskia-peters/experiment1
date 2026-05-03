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

// CreateBalancedGroups creates groups with balanced distribution.
// The distribution strategy is controlled by cfg.Verteilung.Verteilungsmodus:
//   - "Klassisch" (default): max_groesse-bounded groups, no vehicle assignment
//   - "Fahrzeuge": one vehicle per group, vehicle-first algorithm
//   - "FixGroupSize": fixed target group size with optional CarGroups vehicle pooling
//
// Returns a non-empty warning string when distribution succeeded but some
// constraint could not be fully satisfied.
func CreateBalancedGroups(db *sql.DB, cfg config.Config) (string, error) {
	modus := cfg.Verteilung.Verteilungsmodus
	switch modus {
	case "FixGroupSize":
		return createGroupsFixGroupSize(db, cfg)
	case "Fahrzeuge":
		return createGroupsFahrzeuge(db, cfg.Gruppen.MaxGroesse, cfg.Gruppen.MinGroesse)
	default:
		// "Klassisch" and any unrecognised value
		return createGroupsKlassisch(db, cfg.Gruppen.MaxGroesse, cfg.Gruppen.MinGroesse)
	}
}

// createGroupsKlassisch is the no-vehicle distribution path.
// When vehicles are present in the database it automatically falls through to
// createGroupsFahrzeuge, preserving the historical auto-detection behaviour.
func createGroupsKlassisch(db *sql.DB, maxGroupSize int, minGroupSize int) (string, error) {
	// Peek at vehicles — if any are loaded, delegate to the vehicle-first path.
	fahrzeuge, err := database.GetAllFahrzeuge(db)
	if err != nil {
		return "", fmt.Errorf("failed to read fahrzeuge: %w", err)
	}
	if len(fahrzeuge) > 0 {
		return createGroupsFahrzeuge(db, maxGroupSize, minGroupSize)
	}

	teilnehmende, err := database.GetAllTeilnehmende(db)
	if err != nil {
		return "", fmt.Errorf("failed to read teilnehmende: %w", err)
	}
	if len(teilnehmende) == 0 {
		return "", nil
	}
	if err := validatePreGroups(teilnehmende, maxGroupSize); err != nil {
		return "", err
	}
	betreuende, err := database.GetAllBetreuende(db)
	if err != nil {
		return "", fmt.Errorf("failed to read betreuende: %w", err)
	}

	groups := distributeIntoGroups(teilnehmende, maxGroupSize)

	if err := database.SaveGroups(db, groups); err != nil {
		return "", fmt.Errorf("failed to save groups: %w", err)
	}

	var warnings []string
	if len(betreuende) > 0 {
		w, err := distributeBetreuende(groups, betreuende)
		if err != nil {
			return "", err
		}
		if w != "" {
			warnings = append(warnings, w)
		}
		// Enforce min ≥ 2 and max−min ≤ 1. No vehicles → no car drivers, all
		// groups are their own "pool" (trivially no cross-pool constraint).
		poolByGroupID := make(map[int]int, len(groups))
		for _, g := range groups {
			poolByGroupID[g.GroupID] = g.GroupID
		}
		if rw := rebalanceBetreuendeGlobal(groups, nil, poolByGroupID); rw != "" {
			warnings = append(warnings, rw)
		}
		if err := database.SaveGroupBetreuende(db, groups); err != nil {
			return "", fmt.Errorf("failed to save group betreuende: %w", err)
		}
	}

	fmt.Printf("Created %d groups with balanced distribution\n", len(groups))
	for i, group := range groups {
		fmt.Printf("  Group %d: %d participants\n", i+1, len(group.Teilnehmende))
	}
	return strings.Join(warnings, "\n"), nil
}

// createGroupsFahrzeuge is the vehicle-first distribution path.
func createGroupsFahrzeuge(db *sql.DB, maxGroupSize int, minGroupSize int) (string, error) {
	// Read all participants from database
	teilnehmende, err := database.GetAllTeilnehmende(db)
	if err != nil {
		return "", fmt.Errorf("failed to read teilnehmende: %w", err)
	}

	if len(teilnehmende) == 0 {
		return "", nil // No participants to group
	}

	// Reject distribution if any pre-group exceeds the configured group size
	if err := validatePreGroups(teilnehmende, maxGroupSize); err != nil {
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

	var groups []models.Group
	var warnings []string

	if len(fahrzeuge) == 0 {
		if minGroupSize > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"ℹ️ Keine Fahrzeuge vorhanden – min_groesse=%d wird ignoriert, Gruppen werden nach max_groesse=%d gebildet",
				minGroupSize, maxGroupSize))
		}
		groups = distributeIntoGroups(teilnehmende, maxGroupSize)

		if err := database.SaveGroups(db, groups); err != nil {
			return "", fmt.Errorf("failed to save groups: %w", err)
		}

		if len(betreuende) > 0 {
			w, err := distributeBetreuende(groups, betreuende)
			if err != nil {
				return "", err
			}
			if w != "" {
				warnings = append(warnings, w)
			}
			if err := database.SaveGroupBetreuende(db, groups); err != nil {
				return "", fmt.Errorf("failed to save group betreuende: %w", err)
			}
		}
	} else {
		// ── Vehicle-first path ─────────────────────────────────────────────────────
		preGroupMap, unassigned := separateByPreGroup(teilnehmende)

		// Phase 0: sort vehicles, split into eligible / excluded.
		sorted := make([]models.Fahrzeug, len(fahrzeuge))
		copy(sorted, fahrzeuge)
		sort.Slice(sorted, func(i, j int) bool {
			if sorted[i].Ortsverband != sorted[j].Ortsverband {
				return sorted[i].Ortsverband < sorted[j].Ortsverband
			}
			return sorted[i].Bezeichnung < sorted[j].Bezeichnung
		})

		var eligibleFahrzeuge, excludedFahrzeuge []models.Fahrzeug
		for _, f := range sorted {
			if minGroupSize > 0 && f.Sitzplaetze-1 < minGroupSize {
				excludedFahrzeuge = append(excludedFahrzeuge, f)
			} else {
				eligibleFahrzeuge = append(eligibleFahrzeuge, f)
			}
		}
		if len(eligibleFahrzeuge) == 0 {
			return "", fmt.Errorf(
				"alle Fahrzeuge haben zu wenig Sitzplätze (min_groesse=%d) – Verteilung nicht möglich",
				minGroupSize)
		}

		numGroups := len(eligibleFahrzeuge)

		// Further cap numGroups so every group can hold at least minGroupSize TN.
		// Excess eligible vehicles are reported as unused in Phase 6b.
		var countLimitedFahrzeuge []models.Fahrzeug
		if minGroupSize > 0 && len(teilnehmende) > 0 {
			maxGroupsByTN := len(teilnehmende) / minGroupSize
			if maxGroupsByTN < 1 {
				maxGroupsByTN = 1
			}
			if numGroups > maxGroupsByTN {
				countLimitedFahrzeuge = eligibleFahrzeuge[maxGroupsByTN:]
				eligibleFahrzeuge = eligibleFahrzeuge[:maxGroupsByTN]
				numGroups = maxGroupsByTN
			}
		}

		groups = make([]models.Group, numGroups)
		for i := range groups {
			groups[i] = models.Group{
				GroupID:      i + 1,
				Teilnehmende: make([]models.Teilnehmende, 0, maxGroupSize),
				Ortsverbands: make(map[string]int),
				Geschlechts:  make(map[string]int),
			}
		}

		// Phase 1: assign eligible vehicles 1:1 to groups (already sorted).
		vehicleWarn, usedAsDriver := distributeVehicles(groups, eligibleFahrzeuge, betreuende)
		if vehicleWarn != "" {
			warnings = append(warnings, vehicleWarn)
		}

		// Phase 2: fill participants.
		// At this point each group has at most 1 Betreuende (the driver), so
		// effectiveCapacity = min(maxGroupSize, seats−1) — the full minGroupSize
		// worth of TN slots is available.
		if err := fillParticipants(groups, preGroupMap, unassigned, maxGroupSize, &warnings); err != nil {
			return "", err
		}

		// Phase 3: assign remaining Betreuende (non-drivers) AFTER participants.
		// Moving this after fillParticipants prevents non-driver Betreuende from
		// consuming TN seats and causing under-sized groups.
		var remainingBetreuende []models.Betreuende
		for _, b := range betreuende {
			if !usedAsDriver[b.ID] {
				remainingBetreuende = append(remainingBetreuende, b)
			}
		}
		if len(remainingBetreuende) > 0 {
			bWarn, err := distributeBetreuende(groups, remainingBetreuende)
			if err != nil {
				return "", err
			}
			if bWarn != "" {
				warnings = append(warnings, bWarn)
			}
		}

		// Phase 3b: if a vehicle in a group has no resolved driver, assign the
		// group's first licensed Betreuende as the driver.
		for i := range groups {
			for j := range groups[i].Fahrzeuge {
				v := &groups[i].Fahrzeuge[j]
				driverResolved := false
				if v.FahrerName != "" {
					for _, b := range groups[i].Betreuende {
						if strings.EqualFold(strings.TrimSpace(b.Name), strings.TrimSpace(v.FahrerName)) {
							driverResolved = true
							break
						}
					}
				}
				if !driverResolved {
					// Vehicle has no resolved driver — assign the group's first
					// licensed Betreuende. When the group has exactly one
					// Betreuende and no FahrerName was set, this naturally picks
					// that person (still requires Fahrerlaubnis).
					for _, b := range groups[i].Betreuende {
						if b.Fahrerlaubnis {
							v.FahrerName = b.Name
							break
						}
					}
				}
			}
		}

		// Phase 3c: relieve overloaded groups by moving people to groups that
		// still have spare seats. Result is silent — no user-facing warning.
		relieveOverloadedGroups(groups)

		// Phase 3d: swap a non-driver Betreuende from the group with the highest
		// Betreuende:TN ratio with a TN from the group with the lowest ratio.
		// Total headcount per group is preserved, so seat capacity is unaffected.
		// Result is silent — no user-facing warning.
		rebalanceBetreuendeTNRatio(groups)

		// Phase 3e: enforce min ≥ 2 Betreuende per group and max−min ≤ 1.
		// In Fahrzeuge mode each group has its own car; named vehicle drivers are
		// pinned to their group (pool = group). Cat-B/C Betreuende move freely.
		{
			carDriverNames := make(map[string]bool)
			for _, g := range groups {
				for name := range groupDriverNames(g) {
					carDriverNames[name] = true
				}
			}
			poolByGroupID := make(map[int]int, len(groups))
			for _, g := range groups {
				poolByGroupID[g.GroupID] = g.GroupID
			}
			if rw := rebalanceBetreuendeGlobal(groups, carDriverNames, poolByGroupID); rw != "" {
				warnings = append(warnings, rw)
			}
		}

		// Phase 4: capacity checks (run after all people are placed).
		totalSeats := 0
		for _, f := range eligibleFahrzeuge {
			totalSeats += f.Sitzplaetze
		}
		totalPeople := len(betreuende) + len(teilnehmende)
		if totalSeats < totalPeople {
			warnings = append(warnings, fmt.Sprintf(
				"⚠️ Kapazitätsengpass: %d Personen (inkl. Betreuende), aber nur %d Sitzplätze verfügbar – %d Person(en) können nicht alle untergebracht werden",
				totalPeople, totalSeats, totalPeople-totalSeats))
		}
		if capWarn := checkCapacityWarnings(groups); capWarn != "" {
			warnings = append(warnings, capWarn)
		}

		// Phase 6a: report excluded (too-small) vehicles.
		if len(excludedFahrzeuge) > 0 {
			var names []string
			for _, f := range excludedFahrzeuge {
				names = append(names, fmt.Sprintf("%q (%d Plätze, OV %s)",
					f.Bezeichnung, f.Sitzplaetze, f.Ortsverband))
			}
			warnings = append(warnings, fmt.Sprintf(
				"ℹ️ Ausgeschlossene Fahrzeuge (zu klein, min_groesse=%d): %s",
				minGroupSize, strings.Join(names, "; ")))
		}
		// Report vehicles not used because too few TN for another full group.
		if len(countLimitedFahrzeuge) > 0 {
			var names []string
			for _, f := range countLimitedFahrzeuge {
				names = append(names, fmt.Sprintf("%q (OV %s)", f.Bezeichnung, f.Ortsverband))
			}
			warnings = append(warnings, fmt.Sprintf(
				"ℹ️ Nicht verwendete Fahrzeuge (zu wenig Teilnehmende für min_groesse=%d): %s",
				minGroupSize, strings.Join(names, "; ")))
		}

		// Phase 6b: filter out groups that got no Teilnehmende; re-number the rest.
		var activeGroups []models.Group
		var unusedVehicleNames []string
		for _, g := range groups {
			if len(g.Teilnehmende) > 0 {
				activeGroups = append(activeGroups, g)
			} else {
				for _, f := range g.Fahrzeuge {
					unusedVehicleNames = append(unusedVehicleNames,
						fmt.Sprintf("%q (OV %s)", f.Bezeichnung, f.Ortsverband))
				}
			}
		}
		if len(unusedVehicleNames) > 0 {
			warnings = append(warnings, fmt.Sprintf(
				"ℹ️ Ungenutzte Fahrzeuge (0 Teilnehmende): %s",
				strings.Join(unusedVehicleNames, "; ")))
		}
		for i := range activeGroups {
			activeGroups[i].GroupID = i + 1
		}
		groups = activeGroups

		if err := database.SaveGroups(db, groups); err != nil {
			return "", fmt.Errorf("failed to save groups: %w", err)
		}
		if err := database.SaveGroupBetreuende(db, groups); err != nil {
			return "", fmt.Errorf("failed to save group betreuende: %w", err)
		}
		if err := database.SaveGroupFahrzeuge(db, groups); err != nil {
			return "", fmt.Errorf("failed to save group fahrzeuge: %w", err)
		}
	}

	fmt.Printf("Created %d groups with balanced distribution\n", len(groups))
	for i, group := range groups {
		fmt.Printf("  Group %d: %d participants\n", i+1, len(group.Teilnehmende))
	}

	return strings.Join(warnings, "\n"), nil
}

// validatePreGroups returns an error if any PreGroup tag has more members than
// maxGroupSize, listing every offending group by name so the user knows exactly
// which rows in the Excel file need to be corrected.
func validatePreGroups(teilnehmende []models.Teilnehmende, maxGroupSize int) error {
	counts := make(map[string]int)
	for _, t := range teilnehmende {
		if t.PreGroup != "" {
			counts[t.PreGroup]++
		}
	}
	var oversized []string
	for name, count := range counts {
		if count > maxGroupSize {
			oversized = append(oversized,
				fmt.Sprintf("%q (%d Mitglieder, Maximum: %d)", name, count, maxGroupSize))
		}
	}
	if len(oversized) == 0 {
		return nil
	}
	sort.Strings(oversized)
	return fmt.Errorf(
		"folgende Vorgruppen überschreiten die maximale Gruppengröße von %d: %s",
		maxGroupSize, strings.Join(oversized, "; "))
}

// distributeIntoGroups distributes participants into balanced groups
func distributeIntoGroups(teilnehmende []models.Teilnehmende, maxGroupSize int) []models.Group {
	if len(teilnehmende) == 0 {
		return nil
	}
	if maxGroupSize < 1 {
		return nil
	}

	// Step 1: Separate participants with and without PreGroup
	preGroupMap := make(map[string][]models.Teilnehmende)
	var unassignedParticipants []models.Teilnehmende

	for _, t := range teilnehmende {
		if t.PreGroup != "" {
			preGroupMap[t.PreGroup] = append(preGroupMap[t.PreGroup], t)
		} else {
			unassignedParticipants = append(unassignedParticipants, t)
		}
	}

	// Step 2: Calculate number of groups needed
	// Count pre-formed groups
	numPreGroups := len(preGroupMap)
	// Calculate how many additional groups needed for unassigned participants
	numAdditionalGroups := int(math.Ceil(float64(len(unassignedParticipants)) / float64(maxGroupSize)))
	numGroups := numPreGroups + numAdditionalGroups

	// Ensure we have at least one group
	if numGroups == 0 {
		numGroups = 1
	}

	// Step 3: Initialize groups
	groups := make([]models.Group, numGroups)
	for i := range groups {
		groups[i] = models.Group{
			GroupID:      i + 1,
			Teilnehmende: make([]models.Teilnehmende, 0, maxGroupSize),
			Ortsverbands: make(map[string]int),
			Geschlechts:  make(map[string]int),
		}
	}

	// Step 4: Assign pre-grouped participants to the first groups
	groupIdx := 0
	for _, preGroupMembers := range preGroupMap {
		// Add all members of this pre-group to the current group
		for _, t := range preGroupMembers {
			addTeilnehmendeToGroup(&groups[groupIdx], t)
		}
		groupIdx++
	}

	// Step 5: Sort unassigned participants for better distribution
	// First by Ortsverband, then by Geschlecht, then by Alter
	sort.Slice(unassignedParticipants, func(i, j int) bool {
		if unassignedParticipants[i].Ortsverband != unassignedParticipants[j].Ortsverband {
			return unassignedParticipants[i].Ortsverband < unassignedParticipants[j].Ortsverband
		}
		if unassignedParticipants[i].Geschlecht != unassignedParticipants[j].Geschlecht {
			return unassignedParticipants[i].Geschlecht < unassignedParticipants[j].Geschlecht
		}
		return unassignedParticipants[i].Alter < unassignedParticipants[j].Alter
	})

	// Step 6: Distribute unassigned participants using round-robin with diversity scoring
	for _, tn := range unassignedParticipants {
		bestGroupIdx := findBestGroup(groups, tn, maxGroupSize)
		addTeilnehmendeToGroup(&groups[bestGroupIdx], tn)
	}

	return groups
}

// findBestGroup finds the best group for a participant based on diversity
func findBestGroup(groups []models.Group, tn models.Teilnehmende, maxGroupSize int) int {
	bestIdx := 0
	bestScore := math.MaxFloat64

	for i, group := range groups {
		// Skip if group is full
		if len(group.Teilnehmende) >= maxGroupSize {
			continue
		}

		// Calculate diversity score (lower is better)
		score := calculateDiversityScore(group, tn)

		// Prefer groups with fewer members
		sizeBonus := float64(len(group.Teilnehmende)) * 0.5

		totalScore := score + sizeBonus

		if totalScore < bestScore {
			bestScore = totalScore
			bestIdx = i
		}
	}

	return bestIdx
}

// calculateDiversityScore calculates how well a participant fits in a group
// Lower score means better diversity
func calculateDiversityScore(group models.Group, tn models.Teilnehmende) float64 {
	if len(group.Teilnehmende) == 0 {
		return 0
	}

	score := 0.0

	// Penalize if Ortsverband is already common in the group
	ortsverbandCount := group.Ortsverbands[tn.Ortsverband]
	score += float64(ortsverbandCount) * 2.0

	// Penalize if Geschlecht is already common in the group
	geschlechtCount := group.Geschlechts[tn.Geschlecht]
	score += float64(geschlechtCount) * 1.5

	// Penalize if Alter is too similar to group average
	if len(group.Teilnehmende) > 0 && tn.Alter > 0 {
		avgAlter := float64(group.AlterSum) / float64(len(group.Teilnehmende))
		alterDiff := math.Abs(float64(tn.Alter) - avgAlter)
		if alterDiff < 2 {
			score += 1.0
		}
	}

	return score
}

// addTeilnehmendeToGroup adds a participant to the group and updates statistics
func addTeilnehmendeToGroup(g *models.Group, t models.Teilnehmende) {
	g.Teilnehmende = append(g.Teilnehmende, t)
	g.Ortsverbands[t.Ortsverband]++
	g.Geschlechts[t.Geschlecht]++
	g.AlterSum += t.Alter
}

// separateByPreGroup splits participants into a map of pre-group name → members
// and a flat slice of participants without a pre-group assignment.
func separateByPreGroup(teilnehmende []models.Teilnehmende) (map[string][]models.Teilnehmende, []models.Teilnehmende) {
	preGroupMap := make(map[string][]models.Teilnehmende)
	var unassigned []models.Teilnehmende
	for _, t := range teilnehmende {
		if t.PreGroup != "" {
			preGroupMap[t.PreGroup] = append(preGroupMap[t.PreGroup], t)
		} else {
			unassigned = append(unassigned, t)
		}
	}
	return preGroupMap, unassigned
}

// effectiveCapacity returns the maximum number of Teilnehmende that can be
// added to g: the smaller of maxGroupSize and (vehicle seats − Betreuende count).
// Groups without vehicles use maxGroupSize as the sole cap.
func effectiveCapacity(g models.Group, maxGroupSize int) int {
	seats := groupTotalSeats(g)
	if seats == 0 {
		return maxGroupSize
	}
	cap := seats - len(g.Betreuende)
	if cap < 0 {
		cap = 0
	}
	if cap < maxGroupSize {
		return cap
	}
	return maxGroupSize
}

// groupTotalSeats returns the total seat count across all vehicles in a group.
// Returns 0 when the group has no vehicles.
func groupTotalSeats(g models.Group) int {
	total := 0
	for _, f := range g.Fahrzeuge {
		total += f.Sitzplaetze
	}
	return total
}

// checkCapacityWarnings returns a warning string for any group whose total
// headcount (Teilnehmende + Betreuende) exceeds its vehicle seat capacity.
// Groups without vehicles are skipped here — missing-vehicle warnings are
// already emitted by distributeVehicles.
func checkCapacityWarnings(groups []models.Group) string {
	var overloaded []string
	for _, g := range groups {
		seats := groupTotalSeats(g)
		if seats == 0 {
			continue // no vehicle assigned; covered by driver/vehicle warnings
		}
		total := len(g.Teilnehmende) + len(g.Betreuende)
		if total > seats {
			overloaded = append(overloaded, fmt.Sprintf(
				"Gruppe %d: %d Personen, aber nur %d Sitzplätze (%d zu viele)",
				g.GroupID, total, seats, total-seats))
		}
	}
	if len(overloaded) == 0 {
		return ""
	}
	sort.Strings(overloaded)
	return "Fahrzeugkapazität überschritten – folgende Gruppen sind übervoll:\n" + strings.Join(overloaded, "\n")
}

// distributeVehicles assigns vehicles to groups using 1:1 sorted assignment.
// fahrzeuge must already be sorted (done by Phase 0 in CreateBalancedGroups).
// Each vehicle's driver (matched by FahrerName + Ortsverband in the betreuende
// list, requiring Fahrerlaubnis=true) is added to the group as a Betreuende.
// Returns a warning string for any vehicle whose driver could not be found, and
// a set of betreuende IDs that were assigned as drivers (so the caller can skip
// them during the regular Betreuende distribution step).
func distributeVehicles(groups []models.Group, fahrzeuge []models.Fahrzeug, betreuende []models.Betreuende) (string, map[int]bool) {
	usedAsDriver := make(map[int]bool)

	// Build a lookup: (lower-case name, lower-case OV) → licensed Betreuende.
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

	// Direct 1:1 assignment: vehicle[i] → group[i].
	// fahrzeuge is pre-sorted by Phase 0 (OV then Bezeichnung).
	var warnParts []string
	for i, v := range fahrzeuge {
		groups[i].Fahrzeuge = append(groups[i].Fahrzeuge, v)

		// Attach the driver as a Betreuende if they can be resolved.
		if v.FahrerName == "" {
			continue
		}
		k := driverKey{
			name: strings.ToLower(strings.TrimSpace(v.FahrerName)),
			ov:   strings.ToLower(strings.TrimSpace(v.Ortsverband)),
		}
		driver, found := driverMap[k]
		if !found {
			warnParts = append(warnParts, fmt.Sprintf(
				"Fahrzeug %q: Fahrer %q (OV %q) nicht in der Betreuende-Liste gefunden",
				v.Bezeichnung, v.FahrerName, v.Ortsverband))
			continue
		}
		if usedAsDriver[driver.ID] {
			// Driver is already assigned to another vehicle – skip silently.
			continue
		}
		groups[i].Betreuende = append(groups[i].Betreuende, *driver)
		usedAsDriver[driver.ID] = true
	}

	return strings.Join(warnParts, "\n"), usedAsDriver
}

// fillParticipants distributes Teilnehmende into the already-initialized groups
// (which may already contain Betreuende and Fahrzeuge).
//
//   - PreGroups are placed using best-fit: the group with the most remaining
//     effectiveCap that can fit the entire PreGroup in one placement, preferring
//     vehicles from the same OV.  Multiple PreGroups may share one vehicle.
//   - Remaining TN are sorted by OV/Geschlecht/Alter and placed using diversity
//     scoring.
//   - If all groups reach effectiveCap, the +1 exception is attempted.  If that
//     also cannot resolve all overflow, TN are placed in the least-full group
//     and Phase 5 will emit a per-group overload warning.
//
// warnings receives any informational/warning messages generated here (e.g. +1
// exception applied).  Returns a non-nil error only when a PreGroup cannot fit
// in any vehicle.
func fillParticipants(
	groups []models.Group,
	preGroupMap map[string][]models.Teilnehmende,
	unassigned []models.Teilnehmende,
	maxGroupSize int,
	warnings *[]string,
) error {
	// Step 1: PreGroup best-fit placement.
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
			remaining := effectiveCapacity(g, maxGroupSize) - len(g.Teilnehmende)
			if remaining < len(members) {
				continue // won't fit
			}
			// Prefer same-OV vehicle; prefer fewer existing TN (smaller sizeBonus).
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
			return fmt.Errorf(
				"Vorgruppe %q (%d Mitglieder) passt in kein verfügbares Fahrzeug — "+
					"bitte ein größeres Fahrzeug bereitstellen oder die Vorgruppe aufteilen",
				key, len(members))
		}
		for _, t := range members {
			addTeilnehmendeToGroup(&groups[bestIdx], t)
		}
	}

	// Step 2: sort unassigned for better diversity.
	sort.Slice(unassigned, func(i, j int) bool {
		if unassigned[i].Ortsverband != unassigned[j].Ortsverband {
			return unassigned[i].Ortsverband < unassigned[j].Ortsverband
		}
		if unassigned[i].Geschlecht != unassigned[j].Geschlecht {
			return unassigned[i].Geschlecht < unassigned[j].Geschlecht
		}
		return unassigned[i].Alter < unassigned[j].Alter
	})

	// Step 3: main distribution — collect overflow when all groups are full.
	var overflow []models.Teilnehmende
	for _, tn := range unassigned {
		idx := findBestGroupWithCapacity(groups, tn, maxGroupSize)
		if idx < 0 {
			overflow = append(overflow, tn)
		} else {
			addTeilnehmendeToGroup(&groups[idx], tn)
		}
	}

	if len(overflow) == 0 {
		return nil
	}

	// Step 4: +1 exception or plain overflow fallback.
	type plusOne struct{ idx, headroom int }
	var eligible []plusOne
	for i, g := range groups {
		seats := groupTotalSeats(g)
		if seats == 0 {
			continue
		}
		vehicleCap := seats - len(g.Betreuende)
		if vehicleCap > maxGroupSize && len(g.Teilnehmende) == maxGroupSize {
			eligible = append(eligible, plusOne{i, vehicleCap - maxGroupSize})
		}
	}
	totalHeadroom := 0
	for _, e := range eligible {
		totalHeadroom += e.headroom
	}

	if totalHeadroom >= len(overflow) {
		// Vehicle seats beyond maxGroupSize absorb all overflow.
		placed := 0
		for _, e := range eligible {
			for h := 0; h < e.headroom && placed < len(overflow); h++ {
				addTeilnehmendeToGroup(&groups[e.idx], overflow[placed])
				placed++
			}
		}
		*warnings = append(*warnings, fmt.Sprintf(
			"ℹ️ +1-Ausnahme angewendet: %d Teilnehmende überschreiten max_groesse=%d, "+
				"passen aber in die verfügbaren Fahrzeugsitzplätze",
			len(overflow), maxGroupSize))
	} else {
		// Cannot fully resolve — place in least-full group; Phase 5 will warn.
		for _, tn := range overflow {
			leastFull := 0
			for i, g := range groups {
				if len(g.Teilnehmende) < len(groups[leastFull].Teilnehmende) {
					leastFull = i
				}
			}
			addTeilnehmendeToGroup(&groups[leastFull], tn)
		}
	}
	return nil
}

// findBestGroupWithCapacity finds the best group for a participant, respecting
// vehicle seat capacity when vehicles are present.
// Returns -1 when all groups are at capacity; the caller handles overflow.
func findBestGroupWithCapacity(groups []models.Group, tn models.Teilnehmende, maxGroupSize int) int {
	bestIdx := -1
	bestScore := math.MaxFloat64

	for i, group := range groups {
		cap := effectiveCapacity(group, maxGroupSize)
		if len(group.Teilnehmende) >= cap {
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

// distributeBetreuende assigns caretakers to groups.
//
// Algorithm:
//
//  1. Phase 1 – Licensed drivers (Fahrerlaubnis=ja) are spread one-per-group.
//     The driver with the fewest licensed peers in the candidate group wins;
//     ties are broken by participant count from the same OV.  This guarantees
//     that no group receives a second licensed driver before every group has
//     at least one.
//
//  2. Phase 2 – Unlicensed Betreuende follow their OV: they are placed in the
//     group that already holds a licensed driver from the same OV.  When the
//     OV's licensed drivers were split across multiple groups (rare), the
//     unlicensed member joins the group with the fewest Betreuende from that
//     OV.  If no licensed driver from the OV exists, the unlicensed member
//     goes to the group that already has any Betreuende from the same OV, or
//     else to the group with the fewest Betreuende overall.
//
//  3. Phase 3 – Safety net: any group still without a Betreuende receives one
//     donated from the group with the largest surplus (licensed preferred so
//     the donor group keeps a licensed driver if possible).
//
//  4. A non-empty warning string is returned when fewer licensed drivers are
//     available than there are groups, or when a group still ends up with no
//     Betreuende at all.
//
// ovRoundRobinOrder reorders licensed Betreuende so that drivers are drawn from
// each OV in turn before taking a second driver from any single OV.
//
// OVs are ordered by driver count descending (most drivers leads each round);
// ties are broken alphabetically. Within each OV drivers are sorted by name.
//
// Example — OV-Alpha 3, OV-Gamma 2, OV-Beta 1:
//
//	Round 1: Alpha1, Gamma1, Beta1
//	Round 2: Alpha2, Gamma2
//	Round 3: Alpha3
func ovRoundRobinOrder(licensed []models.Betreuende) []models.Betreuende {
	if len(licensed) == 0 {
		return licensed
	}
	// Group by OV, sort each OV's slice by name for determinism.
	ovMap := make(map[string][]models.Betreuende)
	for _, b := range licensed {
		ovMap[b.Ortsverband] = append(ovMap[b.Ortsverband], b)
	}
	for ov := range ovMap {
		sort.Slice(ovMap[ov], func(i, j int) bool {
			return ovMap[ov][i].Name < ovMap[ov][j].Name
		})
	}
	// Sort OVs: most-drivers first, then alphabetical.
	ovs := make([]string, 0, len(ovMap))
	for ov := range ovMap {
		ovs = append(ovs, ov)
	}
	sort.Slice(ovs, func(i, j int) bool {
		ci, cj := len(ovMap[ovs[i]]), len(ovMap[ovs[j]])
		if ci != cj {
			return ci > cj
		}
		return ovs[i] < ovs[j]
	})
	// Interleave: take index r from each OV in sorted order.
	result := make([]models.Betreuende, 0, len(licensed))
	for r := 0; ; r++ {
		any := false
		for _, ov := range ovs {
			if r < len(ovMap[ov]) {
				result = append(result, ovMap[ov][r])
				any = true
			}
		}
		if !any {
			break
		}
	}
	return result
}

func distributeBetreuende(groups []models.Group, betreuende []models.Betreuende) (string, error) {
	if len(groups) == 0 {
		return "", nil
	}

	// Split into licensed / unlicensed, sorted for deterministic output
	var licensed, unlicensed []models.Betreuende
	for _, b := range betreuende {
		if b.Fahrerlaubnis {
			licensed = append(licensed, b)
		} else {
			unlicensed = append(unlicensed, b)
		}
	}
	// Sort licensed using OV round-robin so that each round of assignment draws
	// one driver from each OV before taking a second from any OV. This prevents
	// one OV from supplying all drivers while another contributes none.
	licensed = ovRoundRobinOrder(licensed)
	sort.Slice(unlicensed, func(i, j int) bool {
		if unlicensed[i].Ortsverband != unlicensed[j].Ortsverband {
			return unlicensed[i].Ortsverband < unlicensed[j].Ortsverband
		}
		return unlicensed[i].Name < unlicensed[j].Name
	})

	// licCount tracks how many licensed drivers are in each group.
	// Pre-populate from existing Betreuende so that drivers already assigned
	// by distributeVehicles (vehicle-aware path) are accounted for, preventing
	// the phase-1 spread from piling an extra licensed person on top of a driver.
	licCount := make([]int, len(groups))
	for i, g := range groups {
		for _, b := range g.Betreuende {
			if b.Fahrerlaubnis {
				licCount[i]++
			}
		}
	}

	// --- Phase 1: Spread licensed drivers one-per-group ---
	for _, b := range licensed {
		idx := findGroupForLicensed(groups, b.Ortsverband, licCount)
		groups[idx].Betreuende = append(groups[idx].Betreuende, b)
		licCount[idx]++
	}

	// --- Phase 2: Place unlicensed Betreuende with their OV ---
	for _, b := range unlicensed {
		idx := findGroupForUnlicensed(groups, b.Ortsverband)
		groups[idx].Betreuende = append(groups[idx].Betreuende, b)
	}

	// --- Phase 2b: Rebalance unlicensed Betreuende evenly across groups ---
	// Move unlicensed members from the most-loaded group to the least-loaded
	// group until the difference in total Betreuende count is at most 1.
	// This prevents a group from accumulating 3+ Betreuende while another
	// sits at 1. Only unlicensed members are moved so that Phase 1's
	// one-licensed-driver-per-group guarantee is never disturbed.
	//
	// OV co-location preference: when choosing who to move, prefer a person
	// whose OV still has ≥2 members in the source group (so the OV cluster is
	// not fully broken). When choosing where to move, prefer a destination that
	// already has a Betreuende from the same OV.
	for i := 0; i < len(betreuende)+1; i++ {
		maxIdx, minIdx := -1, -1
		maxCount, minCount := 0, math.MaxInt64
		for j, g := range groups {
			n := len(g.Betreuende)
			if n > maxCount {
				maxCount = n
				maxIdx = j
			}
			if n < minCount {
				minCount = n
				minIdx = j
			}
		}
		if maxIdx < 0 || minIdx < 0 || maxCount-minCount <= 1 {
			break
		}
		// Find an unlicensed member to move from the most-loaded group.
		// Prefer a person whose OV has ≥2 unlicensed members in this group
		// (moving leaves the OV still represented here).
		ovCount := make(map[string]int)
		for _, b := range groups[maxIdx].Betreuende {
			if !b.Fahrerlaubnis {
				ovCount[b.Ortsverband]++
			}
		}
		moveIdx := -1
		for k, b := range groups[maxIdx].Betreuende {
			if !b.Fahrerlaubnis && ovCount[b.Ortsverband] >= 2 {
				moveIdx = k
				break
			}
		}
		if moveIdx < 0 {
			// No preferred candidate; fall back to first unlicensed.
			for k, b := range groups[maxIdx].Betreuende {
				if !b.Fahrerlaubnis {
					moveIdx = k
					break
				}
			}
		}
		if moveIdx < 0 {
			// No unlicensed candidate at all; try licensed members that are NOT
			// pinned as vehicle drivers in this group.  In FixGroupSize mode
			// groups have no vehicles yet, so all licensed are moveable.
			// In Klassisch/Fahrzeuge mode we protect designated drivers.
			driverNamesMax := groupDriverNames(groups[maxIdx])
			for k, b := range groups[maxIdx].Betreuende {
				if b.Fahrerlaubnis && !driverNamesMax[strings.ToLower(strings.TrimSpace(b.Name))] {
					moveIdx = k
					break
				}
			}
		}
		if moveIdx < 0 {
			break // all members are pinned vehicle drivers; cannot rebalance further
		}
		b := groups[maxIdx].Betreuende[moveIdx]
		groups[maxIdx].Betreuende = append(
			groups[maxIdx].Betreuende[:moveIdx],
			groups[maxIdx].Betreuende[moveIdx+1:]...)
		// Prefer a destination that already has a Betreuende from the same OV.
		destIdx := -1
		for j, g := range groups {
			if len(g.Betreuende) == minCount {
				for _, existing := range g.Betreuende {
					if existing.Ortsverband == b.Ortsverband {
						destIdx = j
						break
					}
				}
				if destIdx >= 0 {
					break
				}
			}
		}
		if destIdx < 0 {
			destIdx = minIdx
		}
		groups[destIdx].Betreuende = append(groups[destIdx].Betreuende, b)
	}

	// --- Phase 3: Ensure every group has at least one Betreuende ---
	for i := range groups {
		if len(groups[i].Betreuende) > 0 {
			continue
		}
		// Find the donor group with the most Betreuende (must have ≥2 to donate)
		donorIdx := -1
		donorCount := 0
		for j := range groups {
			if j != i && len(groups[j].Betreuende) > donorCount {
				donorCount = len(groups[j].Betreuende)
				donorIdx = j
			}
		}
		if donorIdx < 0 || donorCount < 2 {
			// Only one Betreuende available in any single group – can't safely
			// donate without leaving that group empty. Accept the situation and
			// let the warning cover it.
			continue
		}
		// Move an unlicensed member first (keep licensed driver in donor group
		// if the donor group would end up without one after the move).
		// OV co-location preference: prefer an unlicensed person whose OV still
		// has ≥2 members in the donor (so the OV cluster is not fully broken).
		donorHasMultipleLicensed := licCount[donorIdx] >= 2

		// Count unlicensed-by-OV in donor for preference selection.
		unlicOVCount := make(map[string]int)
		for _, b := range groups[donorIdx].Betreuende {
			if !b.Fahrerlaubnis {
				unlicOVCount[b.Ortsverband]++
			}
		}

		moveIdx := -1
		// Prefer unlicensed whose OV has ≥2 unlicensed in donor.
		for k, b := range groups[donorIdx].Betreuende {
			if !b.Fahrerlaubnis && unlicOVCount[b.Ortsverband] >= 2 {
				moveIdx = k
				break
			}
		}
		// Fall back to any unlicensed.
		if moveIdx < 0 {
			for k, b := range groups[donorIdx].Betreuende {
				if !b.Fahrerlaubnis {
					moveIdx = k
					break
				}
			}
		}
		// Fall back to licensed if no unlicensed found, but only when donor
		// retains at least one licensed driver after the move.
		if moveIdx < 0 && donorHasMultipleLicensed {
			for k, b := range groups[donorIdx].Betreuende {
				if b.Fahrerlaubnis {
					moveIdx = k
					break
				}
			}
		}
		if moveIdx < 0 {
			continue // cannot donate without stranding the donor
		}
		b := groups[donorIdx].Betreuende[moveIdx]
		groups[donorIdx].Betreuende = append(
			groups[donorIdx].Betreuende[:moveIdx],
			groups[donorIdx].Betreuende[moveIdx+1:]...)
		if b.Fahrerlaubnis {
			licCount[donorIdx]--
			licCount[i]++
		}
		groups[i].Betreuende = append(groups[i].Betreuende, b)
	}

	// --- Phase 4: Build warnings ---
	var noBetreuende, missingLicense []string
	for _, g := range groups {
		if len(g.Betreuende) == 0 {
			noBetreuende = append(noBetreuende, fmt.Sprintf("Gruppe %d", g.GroupID))
		} else if !hasLicensedDriver(g.Betreuende) {
			missingLicense = append(missingLicense, fmt.Sprintf("Gruppe %d", g.GroupID))
		}
	}

	var warnings []string
	if len(noBetreuende) > 0 {
		sort.Strings(noBetreuende)
		warnings = append(warnings,
			fmt.Sprintf("Zu wenig Betreuende – keine Betreuenden für: %s", strings.Join(noBetreuende, ", ")))
	}
	if len(missingLicense) > 0 {
		sort.Strings(missingLicense)
		warnings = append(warnings,
			fmt.Sprintf("Keine Betreuende mit Fahrerlaubnis in: %s", strings.Join(missingLicense, ", ")))
	}

	return strings.Join(warnings, "\n"), nil
}

// findGroupForLicensed returns the group index that should receive the next
// licensed Betreuende from the given OV.
//
// Primary criterion: fewest licensed drivers already in the group (enforces
// the one-per-group invariant – no group gets a second driver before all
// groups have at least one).
// Tiebreak: most participants from the same OV (keeps the Betreuende near
// their own members).
func findGroupForLicensed(groups []models.Group, ov string, licCount []int) int {
	bestIdx := 0
	bestLic := math.MaxInt64
	bestOVCount := -1
	for i, g := range groups {
		lc := licCount[i]
		ovc := g.Ortsverbands[ov]
		if lc < bestLic || (lc == bestLic && ovc > bestOVCount) {
			bestIdx = i
			bestLic = lc
			bestOVCount = ovc
		}
	}
	return bestIdx
}

// findGroupForUnlicensed returns the group index that should receive an
// unlicensed Betreuende from the given OV.
//
// Priority order:
//  1. A group that already has a licensed Betreuende from the same OV
//     (prefer the one with fewest total Betreuende from that OV, so that when
//     an OV's drivers were split the unlicensed member joins the smaller side).
//  2. A group that already has any Betreuende from the same OV (unlicensed).
//  3. The group with the fewest Betreuende overall.
func findGroupForUnlicensed(groups []models.Group, ov string) int {
	// Pass 1: group with a licensed driver from the same OV
	sameLicBest := -1
	sameLicFewest := math.MaxInt64
	for i, g := range groups {
		hasOVLic := false
		ovTotal := 0
		for _, b := range g.Betreuende {
			if b.Ortsverband == ov {
				ovTotal++
				if b.Fahrerlaubnis {
					hasOVLic = true
				}
			}
		}
		if hasOVLic && ovTotal < sameLicFewest {
			sameLicBest = i
			sameLicFewest = ovTotal
		}
	}
	if sameLicBest >= 0 {
		return sameLicBest
	}

	// Pass 2: group with any Betreuende from the same OV
	for i, g := range groups {
		for _, b := range g.Betreuende {
			if b.Ortsverband == ov {
				return i
			}
		}
	}

	// Pass 3: group with fewest Betreuende overall
	fewestIdx := 0
	fewest := math.MaxInt64
	for i, g := range groups {
		if len(g.Betreuende) < fewest {
			fewest = len(g.Betreuende)
			fewestIdx = i
		}
	}
	return fewestIdx
}

// groupDriverNames returns the set of lowercased FahrerName values for all
// vehicles assigned to g. Used to identify which Betreuende are drivers so
// they are never moved during rebalancing.
func groupDriverNames(g models.Group) map[string]bool {
	names := make(map[string]bool)
	for _, v := range g.Fahrzeuge {
		if v.FahrerName != "" {
			names[strings.ToLower(strings.TrimSpace(v.FahrerName))] = true
		}
	}
	return names
}

// hasLicensedDriver reports whether any Betreuende in the slice has Fahrerlaubnis.
func hasLicensedDriver(bs []models.Betreuende) bool {
	for _, b := range bs {
		if b.Fahrerlaubnis {
			return true
		}
	}
	return false
}

// relieveOverloadedGroups attempts to move people from groups where headcount
// exceeds vehicle seats into groups that still have spare capacity.
// Teilnehmende are preferred over non-driver Betreuende; drivers are never moved.
// The function iterates until no further moves are possible or needed.
func relieveOverloadedGroups(groups []models.Group) {
	changed := true
	for changed {
		changed = false
		for i := range groups {
			seats := groupTotalSeats(groups[i])
			if seats == 0 {
				continue
			}
			total := len(groups[i].Teilnehmende) + len(groups[i].Betreuende)
			if total <= seats {
				continue
			}

			// Find the target group with the most spare capacity.
			targetIdx := -1
			maxSpare := 0
			for j := range groups {
				if j == i {
					continue
				}
				ts := groupTotalSeats(groups[j])
				if ts == 0 {
					continue
				}
				spare := ts - len(groups[j].Teilnehmende) - len(groups[j].Betreuende)
				if spare > maxSpare {
					maxSpare = spare
					targetIdx = j
				}
			}
			if targetIdx < 0 {
				continue // nowhere to move to
			}

			// Prefer moving a Teilnehmende.
			if len(groups[i].Teilnehmende) > 0 {
				tn := groups[i].Teilnehmende[len(groups[i].Teilnehmende)-1]
				groups[i].Teilnehmende = groups[i].Teilnehmende[:len(groups[i].Teilnehmende)-1]
				groups[i].Ortsverbands[tn.Ortsverband]--
				groups[i].Geschlechts[tn.Geschlecht]--
				groups[i].AlterSum -= tn.Alter
				addTeilnehmendeToGroup(&groups[targetIdx], tn)
				changed = true
				continue
			}

			// Fall back: move a non-driver Betreuende.
			driverNames := groupDriverNames(groups[i])
			for bi, b := range groups[i].Betreuende {
				if driverNames[strings.ToLower(strings.TrimSpace(b.Name))] {
					continue // never move a driver
				}
				groups[i].Betreuende = append(groups[i].Betreuende[:bi], groups[i].Betreuende[bi+1:]...)
				groups[targetIdx].Betreuende = append(groups[targetIdx].Betreuende, b)
				changed = true
				break
			}
		}
	}
}

// rebalanceBetreuendeGlobal enforces two balance guarantees across all groups:
//
//  1. Pass A (hard): every group ends up with ≥ 2 Betreuende.
//     Betreuende are moved from donors (≥ 3) to starved groups (< 2), preferring
//     same-pool donors and unlicensed candidates.
//
//  2. Pass B (soft): the spread of Betreuende counts across groups is ≤ 1
//     (same max−min ≤ 1 goal as the old rebalanceBetreuendeAfterDrivers).
//
// carDriverNames is the set of lowercased-and-trimmed names of named car drivers.
// These people may only be moved to a group within the same pool; if both source
// and destination pools differ they are skipped as candidates.
// To express "never move this driver" (1:1 / Fahrzeuge modes where each group is
// its own pool), set poolByGroupID[g.GroupID] = g.GroupID — a unique pool per
// group ensures canMove always fails for cross-"pool" destinations.
//
// External drivers (IsExternalDriver=true) are never moved regardless of pool.
//
// Each source group must retain ≥ 1 licensed Betreuende after a move; a
// licensed candidate is therefore skipped when it would be the last licensed
// member in its group.
//
// Returns a non-empty warning if min ≥ 2 could not be achieved for some group.
func rebalanceBetreuendeGlobal(
	groups []models.Group,
	carDriverNames map[string]bool,
	poolByGroupID map[int]int,
) string {
	if len(groups) == 0 {
		return ""
	}

	normalise := func(name string) string {
		return strings.ToLower(strings.TrimSpace(name))
	}

	licCountOf := func(idx int) int {
		n := 0
		for _, b := range groups[idx].Betreuende {
			if b.Fahrerlaubnis {
				n++
			}
		}
		return n
	}

	isCarDriver := func(b models.Betreuende) bool {
		return !b.IsExternalDriver && carDriverNames[normalise(b.Name)]
	}

	// canMove returns true if b (in groups[srcIdx]) may be moved to groups[dstIdx].
	canMove := func(srcIdx, dstIdx int, b models.Betreuende) bool {
		if srcIdx == dstIdx {
			return false
		}
		// External drivers are never moved.
		if b.IsExternalDriver {
			return false
		}
		srcPool := poolByGroupID[groups[srcIdx].GroupID]
		dstPool := poolByGroupID[groups[dstIdx].GroupID]
		// Named internal car driver: only within-pool moves.
		if isCarDriver(b) && srcPool != dstPool {
			return false
		}
		// Source group must retain ≥ 1 licensed driver after removal.
		if b.Fahrerlaubnis && licCountOf(srcIdx) <= 1 {
			return false
		}
		return true
	}

	// pickCandidate selects the best betreuende to move from groups[srcIdx] to
	// groups[dstIdx]. Returns (index in Betreuende slice, ok).
	// Priority: unlicensed whose OV has ≥2 in src → any unlicensed → licensed.
	pickCandidate := func(srcIdx, dstIdx int) (int, bool) {
		src := groups[srcIdx].Betreuende
		// Count OV occurrences of unlicensed in src for co-location preference.
		unlicOVCount := make(map[string]int)
		for _, b := range src {
			if !b.Fahrerlaubnis {
				unlicOVCount[b.Ortsverband]++
			}
		}
		// Pass 1: unlicensed with OV count ≥ 2.
		for k, b := range src {
			if !b.Fahrerlaubnis && unlicOVCount[b.Ortsverband] >= 2 && canMove(srcIdx, dstIdx, b) {
				return k, true
			}
		}
		// Pass 2: any unlicensed.
		for k, b := range src {
			if !b.Fahrerlaubnis && canMove(srcIdx, dstIdx, b) {
				return k, true
			}
		}
		// Pass 3: licensed (canMove already ensures src retains ≥ 1 licensed).
		for k, b := range src {
			if b.Fahrerlaubnis && canMove(srcIdx, dstIdx, b) {
				return k, true
			}
		}
		return -1, false
	}

	moveBet := func(srcIdx, dstIdx, betIdx int) {
		b := groups[srcIdx].Betreuende[betIdx]
		groups[srcIdx].Betreuende = append(
			groups[srcIdx].Betreuende[:betIdx],
			groups[srcIdx].Betreuende[betIdx+1:]...)
		groups[dstIdx].Betreuende = append(groups[dstIdx].Betreuende, b)
	}

	// ── Pass A: enforce min ≥ 2 ───────────────────────────────────────────────
	maxIterA := len(groups)*len(groups) + 4*len(groups) + 1
	for iter := 0; iter < maxIterA; iter++ {
		// Find the most-starved group (< 2 betreuende).
		starvedIdx := -1
		starvedMin := 2
		for i, g := range groups {
			n := len(g.Betreuende)
			if n < starvedMin {
				starvedMin = n
				starvedIdx = i
			}
		}
		if starvedIdx < 0 {
			break // all groups have ≥ 2
		}
		dstPool := poolByGroupID[groups[starvedIdx].GroupID]

		moved := false
		// Pass 0: same-pool donors; pass 1: any-pool donors.
		for pass := 0; pass < 2 && !moved; pass++ {
			// Pick donor with most betreuende (≥ 3).
			donorIdx := -1
			donorCount := 2 // threshold: donor must have > 2 to remain ≥ 2 after giving
			for i, g := range groups {
				if i == starvedIdx {
					continue
				}
				srcPool := poolByGroupID[g.GroupID]
				if pass == 0 && srcPool != dstPool {
					continue
				}
				if len(g.Betreuende) > donorCount {
					donorCount = len(g.Betreuende)
					donorIdx = i
				}
			}
			if donorIdx < 0 {
				continue
			}
			if k, ok := pickCandidate(donorIdx, starvedIdx); ok {
				moveBet(donorIdx, starvedIdx, k)
				moved = true
			}
		}
		if !moved {
			break // no suitable donor; remaining starvation is unavoidable
		}
	}

	// Collect groups still below 2 for the warning.
	var stillStarved []string
	for _, g := range groups {
		if len(g.Betreuende) < 2 {
			stillStarved = append(stillStarved, fmt.Sprintf("Gruppe %d (%d Betreuende)", g.GroupID, len(g.Betreuende)))
		}
	}
	var warnings []string
	if len(stillStarved) > 0 {
		sort.Strings(stillStarved)
		warnings = append(warnings, fmt.Sprintf(
			"⚠️ Folgende Gruppen haben weniger als 2 Betreuende (bitte mindestens %d Betreuende importieren): %s",
			2*len(groups), strings.Join(stillStarved, ", ")))
	}

	// ── Pass B: max − min ≤ 1 ─────────────────────────────────────────────────
	maxIterB := len(groups)*len(groups) + 4*len(groups) + 1
	for iter := 0; iter < maxIterB; iter++ {
		maxIdx, minIdx := 0, 0
		for i := range groups {
			if len(groups[i].Betreuende) > len(groups[maxIdx].Betreuende) {
				maxIdx = i
			}
			if len(groups[i].Betreuende) < len(groups[minIdx].Betreuende) {
				minIdx = i
			}
		}
		if len(groups[maxIdx].Betreuende)-len(groups[minIdx].Betreuende) <= 1 {
			break
		}
		k, ok := pickCandidate(maxIdx, minIdx)
		if !ok {
			break
		}
		moveBet(maxIdx, minIdx, k)
	}

	return strings.Join(warnings, "\n")
}

// rebalanceBetreuendeTNRatio swaps a non-driver Betreuende from the group with
// the highest Betreuende:TN ratio with a Teilnehmende from the group with the
// lowest ratio, repeating until no swap reduces the maximum ratio further.
// Because one person moves in each direction per swap, the total headcount per
// group is unchanged, so vehicle seat capacity is automatically preserved.
// Drivers (matched via vehicle FahrerName) are never moved; the donating group
// always retains at least one Betreuende; the receiving group always retains at
// least one Teilnehmende after the swap.
func rebalanceBetreuendeTNRatio(groups []models.Group) {
	// Cap iterations to prevent any accidental infinite loop.
	maxIter := len(groups)*len(groups)*10 + 10
	for iter := 0; iter < maxIter; iter++ {
		bestGain := 0.0
		bestHi, bestLo := -1, -1
		bestBi := -1 // Betreuende index in groups[bestHi] to move

		for hi := range groups {
			tnHi := len(groups[hi].Teilnehmende)
			bHi := len(groups[hi].Betreuende)
			if tnHi == 0 || bHi < 2 {
				continue // need ≥1 TN for ratio; need ≥2 B so one stays behind
			}
			ratioHi := float64(bHi) / float64(tnHi)

			// Find a moveable (non-driver) Betreuende in the high-ratio group.
			driverNames := groupDriverNames(groups[hi])
			moveableBI := -1
			for bi, b := range groups[hi].Betreuende {
				if !driverNames[strings.ToLower(strings.TrimSpace(b.Name))] {
					moveableBI = bi
					break
				}
			}
			if moveableBI < 0 {
				continue // only drivers remain – cannot donate
			}

			for lo := range groups {
				if lo == hi {
					continue
				}
				tnLo := len(groups[lo].Teilnehmende)
				bLo := len(groups[lo].Betreuende)
				if tnLo < 2 {
					continue // lo would end up with 0 TN after giving one away
				}
				ratioLo := float64(bLo) / float64(tnLo)
				if ratioHi <= ratioLo {
					continue // no imbalance in this direction
				}

				// Compute ratios after the swap (1 B: hi→lo, 1 TN: lo→hi).
				newRatioHi := float64(bHi-1) / float64(tnHi+1)
				newRatioLo := float64(bLo+1) / float64(tnLo-1)
				newMax := newRatioHi
				if newRatioLo > newMax {
					newMax = newRatioLo
				}
				gain := ratioHi - newMax // ratioHi is the current max (ratioHi > ratioLo)
				if gain > bestGain {
					bestGain = gain
					bestHi = hi
					bestLo = lo
					bestBi = moveableBI
				}
			}
		}

		if bestHi < 0 || bestGain <= 1e-9 {
			break
		}

		// Perform the swap.
		b := groups[bestHi].Betreuende[bestBi]
		tn := groups[bestLo].Teilnehmende[len(groups[bestLo].Teilnehmende)-1]

		// Move Betreuende bestHi → bestLo.
		groups[bestHi].Betreuende = append(
			groups[bestHi].Betreuende[:bestBi],
			groups[bestHi].Betreuende[bestBi+1:]...)
		groups[bestLo].Betreuende = append(groups[bestLo].Betreuende, b)

		// Move TN bestLo → bestHi.
		tnIdx := len(groups[bestLo].Teilnehmende) - 1
		groups[bestLo].Teilnehmende = groups[bestLo].Teilnehmende[:tnIdx]
		groups[bestLo].Ortsverbands[tn.Ortsverband]--
		groups[bestLo].Geschlechts[tn.Geschlecht]--
		groups[bestLo].AlterSum -= tn.Alter
		addTeilnehmendeToGroup(&groups[bestHi], tn)
	}
}
