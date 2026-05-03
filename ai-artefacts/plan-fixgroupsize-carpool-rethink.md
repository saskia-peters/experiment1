# Plan: FixGroupSize + CarGroups — Full Algorithm Rethink

## Requirements Summary

| Requirement | Decision |
|---|---|
| Every Fahrzeug in xlsx must have a named driver | Hard constraint — hard error if violated |
| Number of betreuende-groups | Same as number of participant groups |
| Drivers per B-group | ≥ 1; multiple allowed (multiple cars possible) |
| Non-driver betreuende go to same-OV driver's B-group first | Yes |
| Within a B-group prefer same-OV betreuende | Yes |
| Min betreuende per B-group | 2 |
| B-group to P-group assignment | 1:1, by load-balance (most seats → most TN) |
| Carpool | 1–3 (B-group + P-group) units; combined seats ≥ combined people |
| Seat check | Sum(all car seats in pool) ≥ Sum(all TN + all Betreuende in pool) |
| Fewer drivers than groups | Hard error |
| OV affinity for B-group ↔ P-group pairing | Not required |

---

## Concept: Units and Pools

A **unit** = 1 participant group + 1 betreuende-group (permanently paired):
- Participant group: fixed-size group of TN (no change to current distribution)
- Betreuende-group: ≥ 2 betreuende including ≥ 1 driver with a car

A **carpool** = 1–3 units travelling together:
- Total car seats (all cars of all B-groups in the pool) ≥ total people (all TN + all betreuende in the pool)
- Minimise empty seats; max 3 units per pool

---

## Algorithm

### Phase 0 — Participant distribution (unchanged)

`distributeIntoGroups` / `findBestGroupFixSize` already work correctly. No changes.

---

### Phase 1 — Validate drivers

For every Fahrzeug in the database:
1. FahrerName must be non-empty → hard error otherwise
2. FahrerName must match (case-insensitive, trimmed) a Betreuende in the Betreuende list → hard error if not found
3. No Betreuende may appear as driver for more than 1 car → hard error (duplicate driver)
4. Number of distinct matched drivers must be ≥ numGroups → hard error otherwise

Returns: `driverByCarID map[int]*models.Betreuende` (car ID → its Betreuende driver)

---

### Phase 2 — Form betreuende-groups

**Input:** `numGroups`, all betreuende, all fahrzeuge + their matched drivers.

**2a — Initial driver assignment (1 driver per B-group using OV round-robin)**

Apply the existing `ovRoundRobinOrder` logic but only for the first `numGroups` drivers:
- Sort drivers by OV count descending, then alphabetically (most-drivers-OV leads each round)
- Take the first `numGroups` elements: assign driver[0] → B-group[0], driver[1] → B-group[1], …
- Each B-group gets its driver's car(s)

**2b — Assign remaining drivers (when numDrivers > numGroups)**

For each extra driver d (in the remaining order):
- Find the B-group that contains the most other drivers from the same OV as d (OV co-location)
- Tiebreak: B-group with fewest total betreuende
- Add d and their car(s) to that B-group

**2c — Ensure min-2: add one non-driver to each B-group that has only 1 betreuende**

For each B-group with exactly 1 betreuende (only 1 driver):
- Prefer a non-driver from the same OV as the driver
- Tiebreak: any non-driver (fewest-betreuende-B-group priority among candidates with same OV score)
- Hard constraint: source non-driver must exist; if no non-driver available at all and B-group has
  only 1 betreuende, emit hard error ("not enough betreuende for min-2 constraint")

**2d — Distribute remaining non-drivers**

For each remaining non-driver betreuende (sorted by OV then name):
- Prefer B-group that already has a driver from the same OV
- Tiebreak: B-group with fewest total betreuende (even distribution)
- Assign

---

### Phase 3 — Pair B-groups with P-groups (1:1 by load-balance)

1. Sort P-groups by TN count descending (largest first)
2. Sort B-groups by total car seats descending (most seats first)
3. Pair by index: P-group[i] ↔ B-group[i]
4. Assign B-group's betreuende and fahrzeuge to P-group's `models.Group`

---

### Phase 4 — Form carpools

Uses depth-first backtracking, same structure as `solvePoolCover` but simplified:
- A "unit" headcount = len(group.Teilnehmende) + len(group.Betreuende)
- A "unit" seat total = sum of Sitzplaetze of group.Fahrzeuge
- Pool headcount = sum of unit headcounts for all units in pool
- Pool seats = sum of unit seat totals for all units in pool
- Valid pool: pool seats ≥ pool headcount AND len(units) ≤ 3
- Score: fewest empty seats (seats − headcount); tiebreak: fewest units (prefer larger carpools to save cars)

Each `models.CarGroup` stores:
- `.Groups` = the constituent `models.Group` entries
- `.Cars` = union of `.Fahrzeuge` from all groups (for PDF output)

---

## Changes to Existing Code

### `distribution_fixgroupsize.go`

| What | Change |
|---|---|
| `buildDriverMap` | Replace with new `validateAndMapDrivers` (strict: no free-betreuende concept) |
| `assignCarGroups` | Replace entirely with `formBetreuendeGroupsAndCarpools` |
| `assignVehiclesOneToOne` | No change (unaffected, for cargroups=nein) |
| `createGroupsFixGroupSize` | Steps 5, 7, 8, 9 rewritten: new B-group formation + new carpool solver |

### `distribution.go`

No changes required (participant distribution, Klassisch, Fahrzeuge modes untouched).

### `models/types.go`

No new types needed. `models.CarGroup.Cars` will be set to the union of groups' Fahrzeuge.

---

## New Function: `validateAndMapDrivers`

```
func validateAndMapDrivers(fahrzeuge []models.Fahrzeug, betreuende []models.Betreuende, numGroups int) (
    driverByCarID map[int]*models.Betreuende,
    warnings []string,
    err error,
)
```

Errors (hard):
- Any Fahrzeug has empty FahrerName
- Any FahrerName not found in Betreuende list
- Same Betreuende named as driver for 2+ cars
- Distinct driver count < numGroups

Warnings (soft):
- OV string mismatch between Fahrzeug.Ortsverband and Betreuende.Ortsverband (informational, still accepted)

---

## New Function: `formBetreuendeGroups`

```
func formBetreuendeGroups(
    numGroups int,
    betreuende []models.Betreuende,
    fahrzeuge []models.Fahrzeug,
    driverByCarID map[int]*models.Betreuende,
) (bgroups []bGroup, warnings string, err error)

type bGroup struct {
    betreuende []models.Betreuende
    fahrzeuge  []models.Fahrzeug
    totalSeats int
}
```

Implements Phases 2a–2d from the algorithm above.

---

## New Function: `pairAndAssignUnits`

```
func pairAndAssignUnits(groups []models.Group, bgroups []bGroup)
```

Implements Phase 3: sort P-groups by TN count, sort B-groups by total seats, pair by index,
write Betreuende + Fahrzeuge into each `models.Group`.

---

## New Function: `solveUnitCarpools`

```
func solveUnitCarpools(groups []models.Group) []*models.CarGroup
```

Implements Phase 4: depth-first cover of all groups into pools of 1–3 units.
Each pool: `models.CarGroup{Groups: ..., Cars: union of groups' Fahrzeuge}`.

---

## Removed Complexity

The following are no longer needed in the FixGroupSize+cargroups path:
- Fallback driver assignment (empty FahrerName → licensed betreuende) — hard error instead
- External driver synthetic betreuende entries
- The `distributeBetreuende` call in FixGroupSize+cargroups path (replaced by B-group formation)
- `rebalanceBetreuendeGlobal` call in FixGroupSize+cargroups path (B-group formation guarantees min-2 directly)
- `buildDriverMap`'s "free betreuende" concept

---

## Updated `createGroupsFixGroupSize` Steps

```
Step 1: compute numGroups (unchanged)
Step 2: compute per-group capacities (unchanged)
Step 3: initialise groups (unchanged)
Step 4: distribute TN (unchanged)
Step 5: validate drivers → hard error if any constraint fails
Step 6: form B-groups (Phases 2a–2d)
Step 7: save TN assignments
Step 8: pair B-groups to P-groups (Phase 3)
Step 9: form carpools (Phase 4)
Step 10: save Betreuende + Fahrzeuge + CarGroups
```

---

## Test Cases

| Scenario | Expected |
|---|---|
| 4 groups, 4 drivers (1 per OV), 4 non-drivers | 4 B-groups each with 1 driver + 1 non-driver |
| 4 groups, 6 drivers (OV-Alpha 4, OV-Beta 2) | B-group gets 2 Alpha drivers; 2 B-groups have 2 cars |
| 4 groups, 3 drivers | Hard error |
| Fahrzeug with empty FahrerName | Hard error |
| Driver name not found in Betreuende | Hard error |
| 4 groups, 4 drivers, 0 non-drivers | B-groups with 1 betreuende each → hard error (min-2 impossible) |
| 4 groups, 4 drivers, 4 non-drivers, 2 from same OV as one driver | Same-OV non-drivers go to matching B-group |
| 3 units, smallest pool: 2 units fit but 3 don't due to seats | 2-unit pool formed; remaining unit is solo pool |

---

## Open Questions

| Question | Decision |
|---|---|
| Can a Betreuende be driver for 2 cars? | Hard error (physically impossible) |
| Is `distributeBetreuende` still called for FixGroupSize+cargroups? | No — replaced by B-group formation |
| Is `rebalanceBetreuendeGlobal` still called for FixGroupSize+cargroups? | No — B-group formation guarantees min-2 directly |
| Does cargroups=nein path change? | No — `assignVehiclesOneToOne` unchanged |
