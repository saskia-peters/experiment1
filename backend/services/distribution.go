package services

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
	"strings"

	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/models"
)

// CreateBalancedGroups creates groups with balanced distribution.
// maxGroupSize controls the maximum number of participants per group.
// Returns a non-empty warning string when distribution succeeded but the
// Fahrerlaubnis or vehicle-capacity constraint could not be fully satisfied.
func CreateBalancedGroups(db *sql.DB, maxGroupSize int) (string, error) {
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
		// ── Original path (no vehicles) ────────────────────────────────────────────
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
		// ── Vehicle-aware path ─────────────────────────────────────────────────────
		// Phase 0: determine group count and create empty group shells.
		preGroupMap, unassigned := separateByPreGroup(teilnehmende)
		numPreGroups := len(preGroupMap)
		numAdditional := int(math.Ceil(float64(len(unassigned)) / float64(maxGroupSize)))
		numGroups := numPreGroups + numAdditional
		if numGroups < 1 {
			numGroups = 1
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

		// Phase 1: assign vehicles and their drivers to groups.
		vehicleWarn, usedAsDriver := distributeVehicles(groups, fahrzeuge, betreuende)
		if vehicleWarn != "" {
			warnings = append(warnings, vehicleWarn)
		}

		// Phase 2: assign remaining Betreuende (those not already driving a vehicle).
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

		// Phase 3: upfront capacity check — warn before filling if total seats
		// cannot accommodate all Betreuende + Teilnehmende.
		totalSeats := 0
		for _, f := range fahrzeuge {
			totalSeats += f.Sitzplaetze
		}
		totalPeople := len(betreuende) + len(teilnehmende)
		if totalSeats < totalPeople {
			warnings = append(warnings, fmt.Sprintf(
				"⚠️ Kapazitätsengpass: %d Personen (inkl. Betreuende), aber nur %d Sitzplätze verfügbar – %d Person(en) können nicht alle untergebracht werden",
				totalPeople, totalSeats, totalPeople-totalSeats))
		}

		// Phase 4: fill participants, respecting vehicle seat capacity as hard limit.
		fillParticipants(groups, preGroupMap, unassigned, maxGroupSize)

		// Phase 5: per-group capacity check (detail for the operator).
		if capWarn := checkCapacityWarnings(groups); capWarn != "" {
			warnings = append(warnings, capWarn)
		}

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

// distributeVehicles assigns vehicles to groups before participants are added.
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

	// Sort vehicles deterministically: by OV then by Bezeichnung.
	sorted := make([]models.Fahrzeug, len(fahrzeuge))
	copy(sorted, fahrzeuge)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Ortsverband != sorted[j].Ortsverband {
			return sorted[i].Ortsverband < sorted[j].Ortsverband
		}
		return sorted[i].Bezeichnung < sorted[j].Bezeichnung
	})

	var warnParts []string
	for _, v := range sorted {
		// Assign vehicle to the group with the fewest total seats so far
		// (balances capacity). OV participant count breaks ties.
		idx := findGroupForVehicle(groups, v.Ortsverband)
		groups[idx].Fahrzeuge = append(groups[idx].Fahrzeuge, v)

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
			// Driver is already assigned to another vehicle – this is unusual
			// (one person can only drive one vehicle); skip silently.
			continue
		}
		groups[idx].Betreuende = append(groups[idx].Betreuende, *driver)
		usedAsDriver[driver.ID] = true
	}

	return strings.Join(warnParts, "\n"), usedAsDriver
}

// findGroupForVehicle returns the index of the group that should receive the
// next vehicle.  Primary criterion: fewest total vehicle seats so far (balances
// load).  Tiebreak: most existing participants from the vehicle's OV.
func findGroupForVehicle(groups []models.Group, ov string) int {
	bestIdx := 0
	bestSeats := math.MaxInt64
	bestOVCount := -1
	for i, g := range groups {
		seats := groupTotalSeats(g)
		ovc := g.Ortsverbands[ov]
		if seats < bestSeats || (seats == bestSeats && ovc > bestOVCount) {
			bestIdx = i
			bestSeats = seats
			bestOVCount = ovc
		}
	}
	return bestIdx
}

// fillParticipants distributes teilnehmende into the already-initialized groups
// (which may already contain Betreuende and Fahrzeuge).  Pre-grouped
// participants are placed first; the remainder are spread using diversity
// scoring.  Each group's effective capacity is:
//
//	min(maxGroupSize, totalSeats − len(Betreuende))
//
// where totalSeats is the sum of all vehicle seats in that group.  Groups
// without vehicles use maxGroupSize as the sole cap.
func fillParticipants(groups []models.Group, preGroupMap map[string][]models.Teilnehmende, unassigned []models.Teilnehmende, maxGroupSize int) {
	// Step 1: assign pre-grouped members in sorted key order (deterministic).
	preGroupKeys := make([]string, 0, len(preGroupMap))
	for k := range preGroupMap {
		preGroupKeys = append(preGroupKeys, k)
	}
	sort.Strings(preGroupKeys)

	groupIdx := 0
	for _, key := range preGroupKeys {
		for _, t := range preGroupMap[key] {
			addTeilnehmendeToGroup(&groups[groupIdx], t)
		}
		groupIdx++
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

	// Step 3: distribute using diversity scoring with capacity awareness.
	for _, tn := range unassigned {
		idx := findBestGroupWithCapacity(groups, tn, maxGroupSize)
		addTeilnehmendeToGroup(&groups[idx], tn)
	}
}

// findBestGroupWithCapacity finds the best group for a participant, respecting
// vehicle seat capacity when vehicles are present.
func findBestGroupWithCapacity(groups []models.Group, tn models.Teilnehmende, maxGroupSize int) int {
	bestIdx := -1
	bestScore := math.MaxFloat64

	for i, group := range groups {
		// Determine effective capacity for this group.
		seats := groupTotalSeats(group)
		var cap int
		if seats > 0 {
			// Vehicle-constrained: remaining seats after Betreuende are seated.
			cap = seats - len(group.Betreuende)
			if cap < 0 {
				cap = 0
			}
		} else {
			cap = maxGroupSize
		}

		if len(group.Teilnehmende) >= cap || len(group.Teilnehmende) >= maxGroupSize {
			continue
		}

		score := calculateDiversityScore(group, tn)
		sizeBonus := float64(len(group.Teilnehmende)) * 0.5
		total := score + sizeBonus

		if total < bestScore {
			bestScore = total
			bestIdx = i
		}
	}

	// Fallback: all groups are at capacity (vehicles too small for the total
	// headcount). Place in the least-full group so no participant is lost; the
	// capacity warning will surface the overload to the operator.
	if bestIdx < 0 {
		leastFull := 0
		for i, g := range groups {
			if len(g.Teilnehmende) < len(groups[leastFull].Teilnehmende) {
				leastFull = i
			}
		}
		return leastFull
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
	sort.Slice(licensed, func(i, j int) bool {
		if licensed[i].Ortsverband != licensed[j].Ortsverband {
			return licensed[i].Ortsverband < licensed[j].Ortsverband
		}
		return licensed[i].Name < licensed[j].Name
	})
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
		moveIdx := -1
		for k, b := range groups[maxIdx].Betreuende {
			if !b.Fahrerlaubnis {
				moveIdx = k
				break
			}
		}
		if moveIdx < 0 {
			break // only licensed members remain in the max group; cannot rebalance further
		}
		b := groups[maxIdx].Betreuende[moveIdx]
		groups[maxIdx].Betreuende = append(
			groups[maxIdx].Betreuende[:moveIdx],
			groups[maxIdx].Betreuende[moveIdx+1:]...)
		groups[minIdx].Betreuende = append(groups[minIdx].Betreuende, b)
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
		donorHasMultipleLicensed := licCount[donorIdx] >= 2
		moveIdx := -1
		// Prefer unlicensed if donor keeps its licensed driver
		for k, b := range groups[donorIdx].Betreuende {
			if !b.Fahrerlaubnis {
				moveIdx = k
				break
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

// hasLicensedDriver reports whether any Betreuende in the slice has Fahrerlaubnis.
func hasLicensedDriver(bs []models.Betreuende) bool {
	for _, b := range bs {
		if b.Fahrerlaubnis {
			return true
		}
	}
	return false
}
