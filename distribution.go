package main

import (
	"database/sql"
	"fmt"
	"math"
	"sort"
)

// createBalancedGroups creates groups with balanced distribution
func createBalancedGroups(db *sql.DB) error {
	// Read all participants from database
	teilnehmers, err := getAllTeilnehmers(db)
	if err != nil {
		return fmt.Errorf("failed to read teilnehmers: %w", err)
	}

	if len(teilnehmers) == 0 {
		return nil // No participants to group
	}

	// Create balanced groups using the distribution algorithm
	groups := distributeIntoGroups(teilnehmers)

	// Save groups to database
	if err := saveGroups(db, groups); err != nil {
		return fmt.Errorf("failed to save groups: %w", err)
	}

	fmt.Printf("Created %d groups with balanced distribution\n", len(groups))
	for i, group := range groups {
		fmt.Printf("  Group %d: %d participants\n", i+1, len(group.Teilnehmers))
	}

	return nil
}

// distributeIntoGroups distributes participants into balanced groups
func distributeIntoGroups(teilnehmers []Teilnehmer) []Group {
	if len(teilnehmers) == 0 {
		return nil
	}

	// Calculate number of groups needed
	numGroups := int(math.Ceil(float64(len(teilnehmers)) / float64(maxGroupSize)))

	// Initialize groups
	groups := make([]Group, numGroups)
	for i := range groups {
		groups[i] = Group{
			GroupID:      i + 1,
			Teilnehmers:  make([]Teilnehmer, 0, maxGroupSize),
			Ortsverbands: make(map[string]int),
			Geschlechts:  make(map[string]int),
		}
	}

	// Sort participants for better distribution
	// First by Ortsverband, then by Geschlecht, then by Alter
	sort.Slice(teilnehmers, func(i, j int) bool {
		if teilnehmers[i].Ortsverband != teilnehmers[j].Ortsverband {
			return teilnehmers[i].Ortsverband < teilnehmers[j].Ortsverband
		}
		if teilnehmers[i].Geschlecht != teilnehmers[j].Geschlecht {
			return teilnehmers[i].Geschlecht < teilnehmers[j].Geschlecht
		}
		return teilnehmers[i].Alter < teilnehmers[j].Alter
	})

	// Distribute participants using round-robin with diversity scoring
	for _, teilnehmer := range teilnehmers {
		bestGroupIdx := findBestGroup(groups, teilnehmer)
		groups[bestGroupIdx].addTeilnehmer(teilnehmer)
	}

	return groups
}

// findBestGroup finds the best group for a participant based on diversity
func findBestGroup(groups []Group, teilnehmer Teilnehmer) int {
	bestIdx := 0
	bestScore := math.MaxFloat64

	for i, group := range groups {
		// Skip if group is full
		if len(group.Teilnehmers) >= maxGroupSize {
			continue
		}

		// Calculate diversity score (lower is better)
		score := calculateDiversityScore(group, teilnehmer)

		// Prefer groups with fewer members
		sizeBonus := float64(len(group.Teilnehmers)) * 0.5

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
func calculateDiversityScore(group Group, teilnehmer Teilnehmer) float64 {
	if len(group.Teilnehmers) == 0 {
		return 0
	}

	score := 0.0

	// Penalize if Ortsverband is already common in the group
	ortsverbandCount := group.Ortsverbands[teilnehmer.Ortsverband]
	score += float64(ortsverbandCount) * 2.0

	// Penalize if Geschlecht is already common in the group
	geschlechtCount := group.Geschlechts[teilnehmer.Geschlecht]
	score += float64(geschlechtCount) * 1.5

	// Penalize if Alter is too similar to group average
	if len(group.Teilnehmers) > 0 && teilnehmer.Alter > 0 {
		avgAlter := float64(group.AlterSum) / float64(len(group.Teilnehmers))
		alterDiff := math.Abs(float64(teilnehmer.Alter) - avgAlter)
		if alterDiff < 2 {
			score += 1.0
		}
	}

	return score
}

// addTeilnehmer adds a participant to the group and updates statistics
func (g *Group) addTeilnehmer(t Teilnehmer) {
	g.Teilnehmers = append(g.Teilnehmers, t)
	g.Ortsverbands[t.Ortsverband]++
	g.Geschlechts[t.Geschlecht]++
	g.AlterSum += t.Alter
}
