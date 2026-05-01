# Plan: CarPool Assignment — Seat-Fit Optimisation

## Problem Statement

The current `assignCarGroups` creates **one pool per participant group** (N groups → N pools,
each group gets its own cars). The desired behaviour is:

> Combine multiple participant groups into shared pools. Each pool gets a set of cars.
> Minimise empty seats per pool, starting with 0 and increasing tolerance until a valid
> assignment is found.

**Example**: 2 groups × 10 persons = 20 people. Cars: 7+7+6 = 20 seats.
→ Both groups form **one pool** with all 3 cars. Zero empty seats. ✅

---

## Confirmed Requirements

| Constraint | Value |
|-----------|-------|
| Groups per pool | 1 – 3 |
| Cars per pool | 1 – 5 |
| OV affinity for pooling | No (pure seat-fit optimisation) |
| Headcount balance between pools | No (only fit to cars matters) |
| Empty seats tolerance | Per-pool; try 0, 1, 2, 3 — hard limit 3 |
| All cars must be used | Yes (excess → tightest pool) |
| Not enough cars | Partial coverage + warning |
| Performance | A few seconds acceptable |
| Scale | ~16 groups, ~20 cars |

---

## Algorithm Design

### Overview

The algorithm runs in three phases:

1. **Candidate generation**: for each possible group combination (1–3 groups), find the best
   car subset (1–5 cars, minimum cars used) that covers the headcount within a given
   tolerance. Rank all candidates by quality.
2. **Greedy cover**: pick the best non-conflicting candidate repeatedly until all groups are
   assigned. If stuck, backtrack one step and try the next candidate.
3. **Remaining cars**: distribute leftover cars (all must be used) to the pool with the
   least spare capacity (seats − people).

---

### Phase 1 — Candidate Generation

#### Inputs
- `groups []models.Group` — with headcounts h[i]
- `cars []models.Fahrzeug` — with seat counts s[j]

#### Group combinations
Enumerate all subsets of 1, 2, or 3 groups:
```
for size in {1, 2, 3}:
    for each size-subset of groups:
        headcount = sum of group headcounts in subset
```

#### Car subset selection (subset-sum DP)
For each group combination with headcount H and tolerance T (0..3):

Find a car subset of size 1–5 from the *remaining* available cars such that:

```
H ≤ sum(car.seats) ≤ H + T   AND   subset size ≤ 5
```

Use a **DP over remaining cars** (max 20):
- `dp[seats] = (minCars, carMask)` — minimum number of cars to reach exactly `seats` seats
- Table size: max 20 cars × ~200 total seats → ~4 000 entries, extremely fast
- After DP: scan `dp[H]`, `dp[H+1]`, ..., `dp[H+T]` for the entry with the fewest cars
- If multiple entries have the same car count, prefer the one with fewer empty seats
- Backtrack through the DP table to recover the actual car subset

#### Candidate record
```go
type poolCandidate struct {
    groups     []int   // indices into groups slice
    cars       []int   // indices into cars slice
    headcount  int
    seats      int
    emptySeats int     // seats - headcount
    carCount   int
}
```

#### Candidate quality ranking (sort key, ascending = better)
1. `emptySeats` (primary — lowest first)
2. `carCount` (secondary — fewest cars used, leaving more for other pools)
3. `len(groups)` descending (tertiary — combine more groups first, reducing pool count)

---

### Phase 2 — Greedy Cover with Backtracking

```
state = { assigned_groups: {}, assigned_cars: {}, pools: [] }
candidates = sorted candidate list (Phase 1, all groups/cars available)

function solve(state, candidates):
    if all groups assigned:
        return SUCCESS
    
    # regenerate valid candidates (filter out used groups/cars)
    valid = [c for c in candidates if no overlap with state]
    if no valid candidates:
        return FAIL
    
    for candidate in valid:    # try best first
        new_state = state + candidate
        if solve(new_state, candidates) == SUCCESS:
            return SUCCESS
    
    return FAIL   # triggers backtrack in caller
```

**Practical bounds** (preventing exponential blowup):
- With 16 groups and max 3 per pool: depth ≤ ceil(16/3) = 6
- At each level: ≤ C(remaining_groups, 1..3) × DP per group combo
- With pruning (sorted, best-first), backtracking is rarely needed; almost always greedy succeeds
- Worst-case depth-first backtracking: still tractable for 16 groups in < 1 second

**Fallback if no valid assignment found at T=3**:
- Log warning and fall back to simple greedy (one group per pool, as before)

---

### Phase 3 — Remaining Cars Distribution

After all groups are covered, some cars may remain (when total cars > minimum needed).

```
for each remaining car (sorted by seats descending):
    find pool with smallest spare_capacity (pool.seats - pool.people)
    add car to that pool
```

This satisfies the user requirement: "extra cars go to the tightest pool first."

---

### Phase 4 — Driver Assignment (unchanged logic)

Same as the current Phase 3b logic in `assignCarGroups`:
- For each car in each pool: if `FahrerName` is blank or not found in the Betreuende list,
  assign the first licensed Betreuende from any group in the pool.

---

## Data Flow

```
createGroupsFixGroupSize
  └── assignCarGroups(groups, fahrzeuge, betreuende) [REPLACE]
        ├── Phase 1: buildCandidates(groups, cars)
        │     └── for each group-combo: subsetSumDP(cars, headcount, tolerance)
        ├── Phase 2: greedyCover(candidates)
        │     └── backtrack on failure
        ├── Phase 3: distributeRemainingCars(pools, remaining cars)
        └── Phase 4: resolveDrivers(pools, betreuende)  [unchanged logic]
```

---

## Code Changes

| File | Function | Change |
|------|----------|--------|
| `backend/services/distribution_fixgroupsize.go` | `assignCarGroups` | Full replacement |
| `backend/services/distribution_fixgroupsize.go` | `subsetSumDP` (new) | DP helper to find minimum car subset within seat range |
| `backend/services/distribution_fixgroupsize.go` | `buildPoolCandidates` (new) | Enumerate and rank all valid (group-combo, car-subset) pairs |
| `backend/services/distribution_fixgroupsize.go` | `greedyCover` (new) | Greedy selection with backtracking |

No changes to:
- `models/types.go` (CarGroup struct unchanged)
- `backend/io/pdf_cargroups.go` (PDF unchanged — it only reads CarGroup.Groups and CarGroup.Cars)
- `distribution.go` (Betreuende distribution unchanged)
- Frontend / Wails bindings

---

## Worked Example

**Input**: 4 groups (10, 10, 8, 7 people), 5 cars (7, 7, 6, 5, 5 seats = 30 total seats for 35 people)

Wait, 30 < 35 — not enough seats. But let's use: 4 groups (8, 8, 7, 7 = 30 people), 5 cars (7, 7, 6, 5, 5 = 30 seats).

**Phase 1 candidates (sorted, tolerance = 0)**:

| Groups | Headcount | Cars | Seats | Empty |
|--------|-----------|------|-------|-------|
| G1+G2 | 16 | C1+C2+C3 | 7+7+... | need 7+6+3? | No exact match with 3 cars |
| G1+G2 | 16 | C1+C2+C3 | 7+7+6=20 | 4 | tolerance=4, skip |
| ... | | | | |
| G3+G4 | 14 | C3+C4+C5 | 6+5+5=16 | 2 | tolerance=2 |

This will fall to tolerance=2 before finding valid assignments. That's fine — algorithm iterates.

**Better example**: 2 groups (10, 10 = 20 people), 3 cars (7, 7, 6 = 20 seats)

Phase 1, tolerance=0:
- G1+G2 (20 people), cars C1+C2+C3 (20 seats) → 0 empty seats ✅
- Selected immediately.

---

## Edge Cases

| Scenario | Behaviour |
|----------|-----------|
| More groups than 3×maxPools would allow | Tolerance raised, some pools get ≤3 groups at best fit |
| Tolerance exceeded (>3 empty) for some pool | Partial coverage, warning emitted |
| No cars available for a pool | Pool created with zero cars, warning: "Kein Fahrzeug für Pool N" |
| 1 group, 20 cars | All 20 cars → one pool (if ≤5 cap: warn and use 5, remaining go to same pool via Phase 3? — reconsider: see open question) |
| All groups identical headcount | DP will produce identical candidates; deterministic due to sorting by group index |

---

## Open Questions

1. **Hard cap of 5 cars per pool** — in Phase 3 (remaining cars), should the 5-car cap also
   apply, or can a pool receive unlimited extra cars?
   - If cap applies: remaining cars that can't fit any pool (all at 5) → warning
   - If cap is soft: allow >5 in Phase 3 only

2. **Tolerance for Phase 3 extra cars** — when a remaining car is added to the tightest pool,
   this car will never reduce empty seats (it always adds slack). Is this acceptable, or
   should the cap prevent over-stuffing a pool with cars it doesn't need?

3. **Single-group pools** — should the algorithm prefer combining groups (2–3 per pool) even
   when a 1-group pool would fit a car perfectly with 0 empty seats? Currently: yes, because
   the sort key prefers more groups at same empty-seat count. Confirm?
