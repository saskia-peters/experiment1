# Plan: CarGroups — Driver Seat Capacity Fix

**Date:** 2026-05-02  
**Status:** Draft — open questions below before implementation  
**Affects:** `verteilungsmodus = "FixGroupSize"` + `cargroups = "ja"` only

---

## 1. Bug Description

### Symptom

A pool of 7 TN + 2 Betreuende is assigned a car with 9 seats. After pool
formation, a third person (the car's driver, who is a Betreuende in the xlsx)
needs a seat too. Result: **10 people, 9 seats — pool is over capacity**.

### Root Cause — two interlocking problems

#### Problem A: driver seat is never reserved

`bestCarSubset` uses `cars[ci].Sitzplaetze` as the total capacity for
passengers. But `Sitzplaetze` **includes** the driver's seat. The driver always
occupies one seat regardless of who they are. So the effective **payload** of
any car is `Sitzplaetze − 1` (passenger seats only).

Currently the pool capacity calculation effectively gives away the driver's seat
to passengers, which over-estimates capacity by 1 per car.

#### Problem B: driver-Betreuende is counted in the wrong pool

When `distributeBetreuende` runs, it knows nothing about car assignments. It
places a licensed Betreuende D (who happens to be the named driver of Car X)
into Group H — which ends up in Pool Q.

Later, `assignCarGroups` assigns Car X to Pool P (containing Group G). Phase 3
sets `car.FahrerName = D`. D is now physically in Car X (Pool P), but the seat
calculation for Pool P never counted D — D's seat was counted in Pool Q.

Result: Pool P has `people(G) + 1 (D)` people but was sized only for
`people(G)`.

### Combined effect

Even if D is in the same pool as their car, Problem A still allows the pool to
be 1 seat too small per car. Problems A and B together can silently produce
multi-seat overflows that are only caught at runtime (warning) or not at all.

---

## 2. Proposed Fix

### Core principle

> **A driver always stays with their car. Every car always has exactly one
> driver. The driver's seat must always be reserved.**

Two changes implement this:

1. **Reserve the driver seat in pool capacity**: use `Sitzplaetze − 1` as each
   car's payload capacity everywhere in `bestCarSubset` and `solvePoolCover`.
   This correctly reserves 1 seat per car for the driver — internal or external.

2. **Separate driver-Betreuende from the normal distribution**: Betreuende who
   are identified as car drivers (xlsx `FahrerName` matches `Betreuende.Name`,
   case-insensitive) are **not distributed by `distributeBetreuende`**. They are
   instead assigned to a group within their car's pool *after* pool formation.

---

## 3. New Execution Order

```
OLD order:
  createTNGroups → distributeBetreuende (all) → assignCarGroups → Phase 3 drivers

NEW order:
  createTNGroups
    → identifyDriverBetreuende          (new step)
    → distributeBetreuende (free only)  (modified: excludes driver-Betreuende)
    → assignCarGroups (Sitzplaetze−1)   (modified: payload fix)
    → assignDriversToGroups             (new step: post-pool driver placement)
    → rebalanceBetreuende               (modified: skip anchored drivers)
```

---

## 4. Detailed Steps

### Step 0 — Identify driver-Betreuende pairs

Before any distribution, build a map of anchored drivers:

```
driverMap: map[Fahrzeug.ID] → *Betreuende   (nil if driver is external)
freeBetreuende: []Betreuende                 (everyone not matched)
```

Matching rule (same as current Phase 3):  
`strings.EqualFold(strings.TrimSpace(car.FahrerName), strings.TrimSpace(b.Name))`

A Betreuende matched to multiple cars → error / warning (should not happen).  
A car with no FahrerName → no anchor; fallback driver resolved post-pool (current behaviour).  
A car with FahrerName that does **not** match any Betreuende → external driver (truck licence etc.); the `Sitzplaetze − 1` fix already accounts for their seat.

---

### Step 1 — Create TN groups (unchanged)

Ceiling division + diversity score, same as current implementation.

---

### Step 2 — Distribute free Betreuende only

Call `distributeBetreuende(groups, freeBetreuende)` — the same four-phase
algorithm, but driver-Betreuende have been removed from the input list.

Group headcounts at this point: **TN + free Betreuende** (no drivers yet).

---

### Step 3 — Form car pools with corrected seat capacity

Change in `bestCarSubset`:

```go
// BEFORE
s := cars[ci].Sitzplaetze

// AFTER — reserve 1 seat for the driver
s := cars[ci].Sitzplaetze - 1
if s < 1 { s = 1 }   // guard for 1-seat edge case
```

`maxAvail` is recomputed with the new `s` values accordingly.

No other changes to `solvePoolCover` or `assignCarGroups` Phase 1/2.

Headcount per group combo remains: `TN + free Betreuende` (same as Step 2 output).  
Driver seats are now implicitly reserved by the `Sitzplaetze − 1` formula.

---

### Step 4 — Assign driver-Betreuende to groups (new)

After pool formation, for each car with an internal driver (matched Betreuende):

1. Find which pool the car is in.
2. Assign the driver-Betreuende to the group in that pool with the best OV
   affinity (most TN/Betreuende already from the same OV; tie-break: fewest
   Betreuende to avoid piling up).
3. Add the driver-Betreuende to `group.Betreuende` so they appear in the PDF
   and group view.

After this step, every group's `Betreuende` slice contains both free and driver
Betreuende. The seat demand check uses the correct total.

---

### Step 5 — Fallback driver resolution for unnamed cars (modified Phase 3)

Same as current Phase 3, but the candidate pool is now **only free licensed
Betreuende within the pool** (driver-Betreuende are already assigned in Step 4
and must not be re-assigned).

---

### Step 6 — Rebalancing constraint

Rebalancing (`distributeBetreuende` Phase 2b and Phase 3) may only move:

- Unlicensed Betreuende
- Licensed Betreuende who are **not** anchored to a car

Driver-Betreuende may only move **between groups within the same pool** (not
to a group in a different pool).

---

### Step 7 — Seat verification

Existing post-pool warning check (people > seats) now uses the corrected
headcount = `TN + ALL Betreuende (including drivers)` vs.
`sum(Sitzplaetze)` (full seat count including driver seats, because drivers are
now counted in the numerator). The formulas are:

```
poolDemand = sum(TN + Betreuende) for all groups in pool   ← now includes drivers
poolCapacity = sum(car.Sitzplaetze) for all cars in pool   ← full count, no -1
                                                            (driver seats included)
```

The `−1 per car` adjustment was only needed in the *fitting* phase (Steps 3–4)
to avoid reserving the driver's physical seat for a passenger. In the final
verification, both sides include the driver, so the check is symmetric.

---

## 5. Capacity Formula Summary

| Phase | Metric used | Formula |
|-------|------------|---------|
| Pool fitting (DFS/DP) | Car payload | `Sitzplaetze − 1` |
| Pool fitting (DFS/DP) | Group headcount | `TN + free Betreuende` (no drivers yet) |
| Post-pool verification | Pool demand | `sum(TN + ALL Betreuende)` per pool |
| Post-pool verification | Pool capacity | `sum(Sitzplaetze)` per pool |

---

## 6. Example — Before vs After

**Setup:** 7 TN, 2 free Betreuende (B1, B2), 1 driver-Betreuende (D),  
Car X: 9 seats, FahrerName = D.

### Before (bug)

| Step | State |
|------|-------|
| distributeBetreuende | Group G = {7 TN, B1, B2, D} — 10 people |
| solvePoolCover | headcount(G) = 10, Car X capacity = 9 → no fit → WARNING emitted (but may be ignored) |

…OR (if D gets distributed to a different group H):

| Step | State |
|------|-------|
| distributeBetreuende | Group G = {7 TN, B1, B2} — 9 people; D in Group H |
| solvePoolCover | headcount(G) = 9, Car X capacity = 9 → FITS |
| Phase 3 driver | D assigned to Car X in Pool P (which has Group G, not Group H) |
| Reality | 9 people + D = 10, but pool was fitted for 9 → **silent overflow** |

### After (fix)

| Step | State |
|------|-------|
| identifyDriverBetreuende | driverMap[CarX] = D; freeBetreuende = {B1, B2} |
| distributeBetreuende | Group G = {7 TN, B1, B2} — 9 people |
| solvePoolCover | headcount(G) = 9, Car X **payload** = 9−1 = 8 → **does not fit** |
| DFS tries other combos | looks for a car with payload ≥ 9 (i.e. Sitzplaetze ≥ 10) |
| If no such car exists | warning: "Kapazitätsengpass: 9 Personen, max. Sitzplätze − 1 = 8" |

Operator sees a clear capacity warning and either imports a larger car or
reduces group size.

---

## 7. Affected Files

| File | Change |
|------|--------|
| `backend/services/distribution_fixgroupsize.go` | `createGroupsFixGroupSize`: new Step 0 (identify driver-Betreuende), Step 4 (post-pool driver assignment). Pass only `freeBetreuende` to `distributeBetreuende`. |
| `backend/services/distribution_fixgroupsize.go` | `bestCarSubset`: `s = Sitzplaetze − 1` |
| `backend/services/distribution_fixgroupsize.go` | `solvePoolCover`: no change needed (headcount stays TN + free Betreuende) |
| `backend/services/distribution_fixgroupsize.go` | `assignCarGroups` Phase 3: skip Betreuende already assigned as drivers |
| `backend/services/distribution.go` | `distributeBetreuende`: no signature change needed (caller passes filtered list) |
| `test/services_test.go` | New tests for driver-seat reservation and driver-Betreuende anchoring |

---

## 8. Open Questions

### Q1 — Minimum car size guard

With `Sitzplaetze − 1`, a 1-seat car (e.g. a motorbike entered by mistake) has
payload 0 — it can never carry a passenger. Should such cars be excluded with a
warning (similar to the existing `min_groesse` exclusion), or silently skipped
in the DP?

**Options:**
- (a) Exclude payload-0 cars with a warning, same as `min_groesse` exclusion
- (b) Let them fall through to the "unused cars" warning
- (c) Add a hard minimum: `Sitzplaetze ≥ 2` to be eligible

---

### Q2 — Driver-Betreuende OV mismatch

After pool formation, a driver-Betreuende is assigned to the group in their
pool with the best OV match. What if their OV does not match any group in the
pool at all?

**Options:**
- (a) Assign to the group with fewest Betreuende (current fallback logic)
- (b) Assign to the group with the most TN (largest group, most likely to absorb)
- (c) No group preference — just list them as "pool driver" without group attachment (affects PDF display)

---

### Q3 — Rebalancing after driver assignment

When driver-Betreuende are assigned to groups in Step 4, some groups may end
up with more Betreuende than others. The rebalancing (Phase 2b) can only move
free Betreuende. Should driver-Betreuende be swappable *within their pool*
(between groups in the same pool) if balance requires it?

**Options:**
- (a) No — driver-Betreuende are always anchored to their initial group within the pool
- (b) Yes — they may be moved to a different group within the same pool for balance
- (c) Only if the free-Betreuende rebalancing leaves a group with 0 Betreuende

---

### Q4 — External drivers (FahrerName not in Betreuende list)

These drivers need 1 seat (covered by `Sitzplaetze − 1`), but they are not
in any group. Should they appear anywhere in the PDF or group display?

Currently they are shown as `FahrerName` in the CarGroups PDF car table but not
in any group's Betreuende list. This behaviour is unchanged by this plan.

Confirm: **no change needed for external drivers** other than the seat formula fix?

---

### Q5 — Backwards compatibility: Fahrzeuge mode (cargroups = "nein")

The `Sitzplaetze − 1` change affects `bestCarSubset`, which is only called from
`assignCarGroups` (CarGroups mode). The `assignVehiclesOneToOne` path uses a
separate capacity check. Confirm: **only CarGroups mode is affected**.

---

### Q6 — Warning message for capacity overflow

Currently the post-pool warning says:
> "Fahrzeugpool N: X Personen, aber nur Y Sitzplätze verfügbar"

After the fix the message should clarify that 1 seat per car is reserved for
the driver. Proposed new wording:
> "Fahrzeugpool N: X Personen (inkl. Fahrer), aber nur Y Sitzplätze — bitte
> größere Fahrzeuge importieren oder Gruppengröße reduzieren"

Is this wording acceptable, or should it be different?
