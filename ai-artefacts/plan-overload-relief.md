# Plan: Vehicle-First Group Distribution

**Date:** 2026-04-27  
**Status:** Revised v3 – PreGroup and leftover-vehicle semantics clarified

---

## 1. Problem Statement (Root Cause)

The current algorithm has an inverted priority: it calculates a group count first (based on `ceil(N / maxGroupSize)`) and then tries to distribute vehicles across those groups. This creates a structural mismatch:

- Groups are sized without knowing how many seats each will actually have
- Vehicles are a secondary concern, leading to unpredictable overloads

The correct mental model for this use case is:  
**Each vehicle can transport one group. Therefore the pool of available groups = the pool of available vehicles.**  
Not all vehicles need to be used — unused vehicles are simply leftover (reserved for other tasks).

---

## 2. Current Behaviour (for reference)

| Step | What happens |
|------|-------------|
| Phase 0 | Group count = `ceil(N_unassigned / maxGroupSize)` — ignores vehicle count |
| Phase 1 | Vehicles distributed across groups using a balancing heuristic |
| Phase 2 | Remaining Betreuende distributed |
| Phase 3 | Upfront total-seats vs. total-people warning |
| Phase 4 | `fillParticipants` (respects `min(maxGroupSize, seats−betreuende)` per group; falls back to least-full group if all full) |
| Phase 5 | `checkCapacityWarnings` — detects and reports per-group overloads; no remediation |

---

## 3. New Approach: Vehicle-First Distribution

### 3.1 Algorithm Overview

```
numGroups = len(fahrzeuge)     // one group shell per available vehicle
```

Order of operations (vehicle-aware path only):

1. **Phase 0** – Filter vehicles: exclude any vehicle where `seats − 1 < minGroupSize` (conservative: assumes ≥1 Betreuende per vehicle). If `minGroupSize = 0`, no filtering. Create one group shell per eligible vehicle: `numGroups = len(eligibleFahrzeuge)`. Excluded vehicles are reported in Phase 6 alongside unused vehicles.
2. **Phase 1** – Assign each vehicle 1:1 to a group (sorted deterministically by OV then Bezeichnung); attach each vehicle's driver to its group as a Betreuende.
3. **Phase 2** – Distribute remaining Betreuende across groups.
4. **Phase 3** – Upfront capacity check: `sum(effectiveCap)` vs `len(Teilnehmende)`. Warn if insufficient total capacity.
5. **Phase 4** – Fill Teilnehmende:
   a. For each PreGroup (sorted): find a group that has `effectiveCap - current TN ≥ len(PreGroup)` and place the entire PreGroup there. Multiple PreGroups may share one vehicle. If no group can accommodate a PreGroup in full → **error**.
   b. Distribute unassigned TN using diversity scoring, respecting `effectiveCap` per group.
   c. If all groups reach `effectiveCap` and TN remain, apply the **+1 exception check** (see §3.3).
6. **Phase 5** – Capacity check. Should be clean in normal cases; fires only for genuinely unsolvable situations.
7. **Phase 6** – Emit informational note for unused vehicles (groups with 0 TN).

### 3.2 Effective Capacity Formula

```
effectiveCap(group) = min(maxGroupSize, seats(group) − len(Betreuende(group)))
```

**Important:** `maxGroupSize` is an upper bound, but **vehicle capacity usually dominates** in practice. The configured `maxGroupSize` may be set much higher than any single car can physically carry once betreuende are counted. The formula handles both constraints correctly — the smaller of the two always applies.

When `seats − betreuende ≤ maxGroupSize`: vehicle is the binding constraint, `effectiveCap = seats − betreuende`.  
When `seats − betreuende > maxGroupSize`: size limit is the binding constraint, `effectiveCap = maxGroupSize` (and a +1 may be possible, see §3.3).

### 3.3 The +1 Exception

**Precondition:** only meaningful when `seats(g) − len(Betreuende(g)) > maxGroupSize` for some group g — i.e., the vehicle has unused physical seats beyond the configured size limit. If the vehicle is already the tighter constraint, there is nothing to give and this check does not apply.

When the fallback fires (all groups at `effectiveCap`, TN remain unplaced):

1. Count remaining unplaced TN.
2. Find +1-eligible groups: `seats − betreuende > maxGroupSize` AND `len(Teilnehmende) == maxGroupSize`.
3. If the total +1 headroom across all eligible groups `≥` remaining unplaced TN (100% resolution): place them, emit an informational message.
4. Otherwise: emit a warning; place TN in the least-full group (overflow) so no participant is lost.

### 3.4 PreGroup handling

**Semantics:** A PreGroup tag means "these participants must travel in the same vehicle/group." It does **not** imply they need a dedicated vehicle. Multiple PreGroups may share one vehicle, and a vehicle may also carry unassigned TN alongside a PreGroup.

**Placement (Phase 4a):** For each PreGroup (sorted by name for determinism), find the group with the most remaining `effectiveCap` that can fit the entire PreGroup in one placement. Prefer vehicles from the same OV as the PreGroup's members.

**Validation:**

| Condition | Response |
|-----------|----------|
| Any single PreGroup has more members than `max(effectiveCap)` across all groups | **Error** — PreGroup physically cannot fit in any vehicle; operator must split or get a larger vehicle |
| After Phase 1+2: chosen group's remaining capacity < `len(PreGroup)` | **Error** — PreGroup does not fit in the selected vehicle after Betreuende overhead; operator must reorganise |

No error is raised for having more PreGroups than vehicles.

PreGroup members are **never** moved after placement (hard constraint).

### 3.5 Leftover (unused) vehicles

Groups that end up with 0 Teilnehmende after Phase 4 are unused vehicles. They are **not** an error. A short informational note is emitted (Phase 6):
```
ℹ️ Nicht benötigte Fahrzeuge (für andere Aufgaben verfügbar): Bus 3 (OV Köln), …
```
These groups are **not** saved to the database (or saved as empty groups, to be decided at implementation — lean towards not saving to avoid confusion).

### 3.6 What changes vs. the previous approach

| | Old approach | New approach |
|-|-------------|-------------|
| Group count | `ceil(N / maxGroupSize)` | `len(eligibleFahrzeuge)` |
| Vehicle assignment | Distribute vehicles across groups (heuristic) | 1:1 sorted assignment |
| PreGroup placement | Dedicated first-K slots (one slot per PreGroup) | Best-fit into any group; multiple PreGroups can share a vehicle |
| More PreGroups than vehicles | Would error | Fine — PreGroups share vehicles |
| Unused vehicles | Not possible | Fine — groups with 0 TN are unused vehicles, reported informally |
| Overload root cause | Groups created without vehicle awareness | Eliminated |
| maxGroupSize role | Always the binding cap | Upper bound only; vehicle capacity often dominates |
| +1 exception scope | All groups | Only groups where vehicle has headroom beyond maxGroupSize |
| Small-vehicle exclusion | No concept | Vehicles with `seats ≤ minGroupSize` excluded at Phase 0 |

### 3.7 Remaining unsolvable cases (produce warnings, not errors)

- Total `sum(effectiveCap)` < `len(Teilnehmende)` AND +1 exception cannot fully resolve — warn operator
- Total physical seats < total people (Betreuende + Teilnehmende) — nothing can fix this; warn

### 3.8 Minimum Group Size Filter

**Parameter:** `minGroupSize int` (0 = disabled).

A vehicle that physically cannot carry `minGroupSize` Teilnehmende is excluded from the distribution entirely. The check is done in Phase 0, before any Betreuende are assigned, using the conservative estimate of 1 Betreuende per vehicle:

```
exclude vehicle if: seats − 1 < minGroupSize
              i.e.: seats ≤ minGroupSize
```

**Rationale:** Every eligible vehicle gets at least 1 Betreuende (either its driver in Phase 1 or one from Phase 2). Using `seats − 1` is therefore always safe — it will never be more pessimistic than reality.

**What happens to excluded vehicles:**
- They are not used as a group.
- Their named driver (if any) is still treated as a Betreuende and distributed in Phase 2 like any other.
- They are reported in Phase 6 together with unused vehicles:
  ```
  ℹ️ Fahrzeuge zu klein für Mindestgruppengröße (5): MTW O (Bad Säckingen)
  ```

**Edge cases:**

| Condition | Response |
|-----------|----------|
| All vehicles excluded (minGroupSize too high) | Error — no groups can be formed |
| Some PreGroup members would have ridden in an excluded vehicle | No special handling; the PreGroup is placed in a surviving vehicle by Phase 4a |
| `minGroupSize = 0` | No vehicles excluded; behaviour identical to previous |

**2026 dataset impact (minGroupSize = 5):**
- MTW O (Bad Säckingen, 5 seats): `5 − 1 = 4 < 5` → **excluded**
- All other 19 vehicles have ≥ 6 seats → remain eligible
- Net effect: 19 groups instead of 20; Bad Säckingen still has 3 other vehicles (GKW Infra, MTW TZ, Mzkw) with capacity ≥ 5 TN each

---

## 4. Confirmed Decisions

| # | Question | Decision |
|---|----------|---------|
| Q1 | Should PreGroup Teilnehmende ever be moved for capacity relief? | ✅ **Never** – PreGroup is a hard constraint; PreGroup members always stay together in one group |
| Q2 | Can multiple PreGroups share one vehicle? | ✅ **Yes** — PreGroup = "travel together", not "own vehicle" |
| Q3 | What happens to unused vehicles? | ✅ **Not saved to DB** — empty groups are filtered out before `SaveGroups`; informational message in Phase 6 |
| Q4 | Is maxGroupSize always the binding cap? | ✅ **No** — `effectiveCap = min(maxGroupSize, seats − betreuende)`; vehicle capacity often dominates |
| Q5 | +1 exception — when does it apply? | ✅ **Only when vehicle has headroom beyond maxGroupSize** — if the vehicle is already the tighter constraint, no +1 is possible |
| Q6 | Is anything logged persistently? | **Warning/info string only** (consistent with current approach) |
| Q7 (ex-Q5) | Can the root cause be fixed at Phase 0? | ✅ **Yes — this is the new approach.** Deriving group count from vehicles eliminates the root cause. |
| Q8 | What happens to a vehicle too small to carry minGroupSize TN? | ✅ **Excluded from distribution at Phase 0** — not used as a group; its driver (if any) is still distributed as Betreuende in Phase 2; reported in Phase 6 |
| Q9 | Default value for `min_groesse` in config.toml? | ✅ **6** — vehicles with ≤ 6 seats are excluded by default |
| Q10 | Should empty groups be saved to DB? | ✅ **No** — filter out 0-TN groups before `SaveGroups`; re-number remaining groups sequentially |
| Q11 | No-vehicle path: what happens with minGroupSize? | ✅ **Emit informational note** — "min_groesse hat ohne Fahrzeuge keine Wirkung" added to the returned string |
| Q12 | Phase 6 format for excluded vs. unused vehicles? | ✅ **Separate messages** — one line for too-small exclusions, one line for unused (0-TN) vehicles |

---

## 5. Affected Code

| File | Function | Change |
|------|----------|--------|
| `backend/config/config.go` | `GruppenConfig` struct | Add `MinGroesse int \`toml:"min_groesse"\`` |
| `backend/config/config.go` | `Default()` | Set `MinGroesse: 6` |
| `backend/config/config.go` | `defaultTOML` constant | Add `min_groesse = 6` with comment under `[gruppen]` |
| `backend/config/config.go` | `Validate()` | Add check: `MinGroesse < 0 → error` |
| `app_handlers.go` | `App.DistributeGroups()` | Pass `a.cfg.Gruppen.MinGroesse` to `handlers.DistributeGroups` |
| `backend/handlers/files.go` | `DistributeGroups(db, maxGroupSize)` | Add `minGroupSize int` parameter; pass through to `services.CreateBalancedGroups` |
| `backend/services/distribution.go` | `CreateBalancedGroups` | Add `minGroupSize int` parameter; implement Phase 0 filter + Phase 6 exclusion/unused reporting; filter out empty groups before save; re-number remaining groups |
| `backend/services/distribution.go` | `distributeVehicles` | Replace `findGroupForVehicle` heuristic with direct 1:1 index assignment |
| `backend/services/distribution.go` | `fillParticipants` | Change return type to `error`; replace sequential PreGroup placement with best-fit; move overflow/+1 handling here |
| `backend/services/distribution.go` | `findBestGroupWithCapacity` | Remove fallback; return `-1` when no group has remaining capacity |
| `backend/services/distribution.go` | `effectiveCapacity` *(new helper)* | Extract `min(maxGroupSize, seats−betreuende)` logic from `findBestGroupWithCapacity` |
| `backend/services/distribution.go` | `findGroupForVehicle` | **Delete** |
| `test/services_test.go` | *(new)* `TestCreateBalancedGroups_VehicleFirst_*` | Cover all cases in §6 |

**Deleted functions:**
- `findGroupForVehicle` — replaced by 1:1 sorted assignment

No database schema changes. No frontend changes.

---

## 6. Test Cases

| Test name | Setup | Expected outcome |
|-----------|-------|-----------------|
| `_VehicleFirst_GroupCountEqualsVehicleCount` | 3 vehicles, 6 TN, maxGroupSize=8 | 3 groups created; unused groups reported |
| `_VehicleFirst_UnusedVehiclesReported` | 3 vehicles (10-seat each), 1 driver each, 8 TN, maxGroupSize=8 | 1–2 vehicles unused; informational message emitted; unused groups not saved |
| `_VehicleFirst_AllTNFit_NoWarning` | 2 vehicles (5-seat, 10-seat), 1 driver each, 10 TN, maxGroupSize=8 | All 10 TN placed, no overload warning; effectiveCap dominates over maxGroupSize |
| `_VehicleFirst_VehicleCapDominatesOverMaxGroupSize` | 1 vehicle (4-seat), 1 driver, maxGroupSize=8, 3 TN | effectiveCap = min(8, 4−1) = 3; all 3 placed, no warning |
| `_VehicleFirst_PlusOneApplies_VehicleHasHeadroom` | 2 vehicles (9-seat, 9-seat), 1 driver each, maxGroupSize=6, 13 TN | seats−driver=8 > maxGroupSize=6; +1 applied for 1 group, 13 TN placed, informational message |
| `_VehicleFirst_PlusOneNotApplicable_VehicleIsConstraint` | 2 vehicles (6-seat, 6-seat), 1 driver each, maxGroupSize=8, 11 TN | effectiveCap=5 (vehicle is constraint, not maxGroupSize); +1 not applicable; 10 TN placed, 1 warning |
| `_VehicleFirst_MultiplePreGroupsSameVehicle` | 2 vehicles (10-seat, 5-seat), 2 PreGroups (3 TN each), maxGroupSize=8 | Both PreGroups placed in the 10-seat vehicle; no error |
| `_VehicleFirst_PreGroupTooLargeForAnyVehicle` | 2 vehicles (3-seat each), 1 driver each, PreGroup of 3 TN | effectiveCap = min(8, 3−1) = 2 for each; PreGroup of 3 > 2 → error |
| `_VehicleFirst_InsufficientTotalCapacity` | 2 vehicles (4-seat each), 1 driver each, 10 TN, maxGroupSize=8 | Total effectiveCap = 6 < 10; warning emitted |
| `_VehicleFirst_MinGroupSize_SmallVehicleExcluded` | 2 vehicles (5-seat, 8-seat), 1 driver each, minGroupSize=5, 7 TN, maxGroupSize=8 | 5-seat vehicle excluded (`5−1=4 < 5`); 1 group from 8-seat vehicle; all 7 TN placed; excluded vehicle reported |
| `_VehicleFirst_MinGroupSize_AllExcluded` | 2 vehicles (4-seat each), 1 driver each, minGroupSize=5, 6 TN | Both vehicles excluded → error: no groups can be formed |
| `_VehicleFirst_MinGroupSize_Disabled` | 2 vehicles (5-seat, 8-seat), minGroupSize=0, 7 TN | Both vehicles eligible; 2 groups created; normal distribution |

---

## 7. Concrete Implementation

### 7.1 `backend/config/config.go`

**`GruppenConfig` struct** — add `MinGroesse`:
```go
type GruppenConfig struct {
    MaxGroesse   int      `toml:"max_groesse"`
    MinGroesse   int      `toml:"min_groesse"`   // NEW
    Gruppennamen []string `toml:"gruppennamen"`
}
```

**`Default()`** — set default:
```go
Gruppen: GruppenConfig{
    MaxGroesse:   8,
    MinGroesse:   6,   // NEW — vehicles with ≤6 seats excluded by default
    Gruppennamen: DefaultGruppennamen,
},
```

**`defaultTOML`** — add entry after `max_groesse = 8`:
```toml
# Mindestanzahl Teilnehmende pro Gruppe (Fahrzeuge mit zu wenig Plätzen werden ausgeschlossen).
# 0 = kein Mindest (alle Fahrzeuge werden verwendet).
min_groesse = 6
```

**`Validate()`** — add:
```go
if c.Gruppen.MinGroesse < 0 {
    return fmt.Errorf("min_groesse darf nicht negativ sein (aktuell: %d)", c.Gruppen.MinGroesse)
}
```

---

### 7.2 `app_handlers.go`

```go
func (a *App) DistributeGroups() map[string]interface{} {
    return handlers.DistributeGroups(a.db, a.cfg.Gruppen.MaxGroesse, a.cfg.Gruppen.MinGroesse)
}
```

---

### 7.3 `backend/handlers/files.go`

```go
func DistributeGroups(db *sql.DB, maxGroupSize int, minGroupSize int) map[string]interface{} {
    // ...
    warning, err := services.CreateBalancedGroups(db, maxGroupSize, minGroupSize)
    // rest unchanged
}
```

---

### 7.4 `backend/services/distribution.go`

#### 7.4.1 New helper: `effectiveCapacity`

Extract the inline capacity formula into a named helper — used in `findBestGroupWithCapacity`, `fillParticipants`, and Phase 3:

```go
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
```

#### 7.4.2 `CreateBalancedGroups` — new signature and vehicle-aware path

```go
func CreateBalancedGroups(db *sql.DB, maxGroupSize int, minGroupSize int) (string, error) {
```

**No-vehicle path** — add note if minGroupSize > 0:
```go
if len(fahrzeuge) == 0 {
    groups = distributeIntoGroups(teilnehmende, maxGroupSize)
    // ... (save + betreuende, unchanged)
    if minGroupSize > 0 {
        warnings = append(warnings,
            fmt.Sprintf("ℹ️ min_groesse=%d hat ohne Fahrzeuge keine Wirkung", minGroupSize))
    }
}
```

**Vehicle-aware path — Phase 0** (replaces existing Phase 0 entirely):

```go
// Phase 0: filter vehicles and create one group shell per eligible vehicle.
sort.Slice(fahrzeuge, func(i, j int) bool {
    if fahrzeuge[i].Ortsverband != fahrzeuge[j].Ortsverband {
        return fahrzeuge[i].Ortsverband < fahrzeuge[j].Ortsverband
    }
    return fahrzeuge[i].Bezeichnung < fahrzeuge[j].Bezeichnung
})

var eligibleFahrzeuge, excludedFahrzeuge []models.Fahrzeug
for _, f := range fahrzeuge {
    if minGroupSize > 0 && f.Sitzplaetze-1 < minGroupSize {
        excludedFahrzeuge = append(excludedFahrzeuge, f)
    } else {
        eligibleFahrzeuge = append(eligibleFahrzeuge, f)
    }
}
if len(eligibleFahrzeuge) == 0 {
    return "", fmt.Errorf(
        "alle %d Fahrzeuge wurden durch die Mindestgruppengröße (%d) ausgeschlossen — keine Gruppen möglich",
        len(fahrzeuge), minGroupSize)
}

numGroups := len(eligibleFahrzeuge)
groups = make([]models.Group, numGroups)
for i := range groups {
    groups[i] = models.Group{
        GroupID:      i + 1,
        Teilnehmende: make([]models.Teilnehmende, 0),
        Ortsverbands: make(map[string]int),
        Geschlechts:  make(map[string]int),
    }
}
```

**Phase 1** — pass sorted eligible vehicles only:
```go
vehicleWarn, usedAsDriver := distributeVehicles(groups, eligibleFahrzeuge, betreuende)
```

**Phase 3** — update to use `eligibleFahrzeuge` for seat count:
```go
totalSeats := 0
for _, f := range eligibleFahrzeuge {   // was: fahrzeuge
    totalSeats += f.Sitzplaetze
}
```

**Phase 4** — `fillParticipants` now returns `error`:
```go
if err := fillParticipants(groups, preGroupMap, unassigned, maxGroupSize, &warnings); err != nil {
    return "", err
}
```
_(The `&warnings` approach lets `fillParticipants` append the +1 info message without changing its core return type.)_

**Phase 6** — after Phase 5, before saving:
```go
// Phase 6a: report excluded (too-small) vehicles.
if len(excludedFahrzeuge) > 0 {
    names := make([]string, len(excludedFahrzeuge))
    for i, f := range excludedFahrzeuge {
        names[i] = fmt.Sprintf("%s (%s, %d Sitzplätze)", f.Bezeichnung, f.Ortsverband, f.Sitzplaetze)
    }
    sort.Strings(names)
    warnings = append(warnings,
        fmt.Sprintf("ℹ️ Fahrzeuge zu klein für Mindestgruppengröße (%d): %s",
            minGroupSize, strings.Join(names, ", ")))
}

// Phase 6b: filter out unused groups (0 TN); report them; re-number survivors.
var usedGroups []models.Group
var unusedNames []string
for _, g := range groups {
    if len(g.Teilnehmende) == 0 {
        if len(g.Fahrzeuge) > 0 {
            unusedNames = append(unusedNames,
                fmt.Sprintf("%s (%s)", g.Fahrzeuge[0].Bezeichnung, g.Fahrzeuge[0].Ortsverband))
        }
    } else {
        usedGroups = append(usedGroups, g)
    }
}
// Re-number sequentially so GroupID matches the saved index.
for i := range usedGroups {
    usedGroups[i].GroupID = i + 1
}
if len(unusedNames) > 0 {
    sort.Strings(unusedNames)
    warnings = append(warnings,
        "ℹ️ Nicht benötigte Fahrzeuge (für andere Aufgaben verfügbar): "+strings.Join(unusedNames, ", "))
}
groups = usedGroups  // only save non-empty groups

// Save only used groups.
if err := database.SaveGroups(db, groups); err != nil { ... }
```

#### 7.4.3 `distributeVehicles` — 1:1 sorted assignment

The vehicles arrive **already sorted** from Phase 0. Replace `findGroupForVehicle` with direct index assignment:

```go
func distributeVehicles(groups []models.Group, fahrzeuge []models.Fahrzeug, betreuende []models.Betreuende) (string, map[int]bool) {
    usedAsDriver := make(map[int]bool)

    // Build driver lookup (unchanged from current).
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

    // NOTE: fahrzeuge is already sorted (done in Phase 0 of CreateBalancedGroups).
    // Direct 1:1 assignment: vehicle[i] → group[i].
    var warnParts []string
    for i, v := range fahrzeuge {
        groups[i].Fahrzeuge = append(groups[i].Fahrzeuge, v)  // was: groups[idx]

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
            continue
        }
        groups[i].Betreuende = append(groups[i].Betreuende, *driver)  // was: groups[idx]
        usedAsDriver[driver.ID] = true
    }

    return strings.Join(warnParts, "\n"), usedAsDriver
}
```

**Delete `findGroupForVehicle`** entirely — no longer referenced.

#### 7.4.4 `fillParticipants` — new signature, PreGroup best-fit, overflow handling

```go
func fillParticipants(
    groups []models.Group,
    preGroupMap map[string][]models.Teilnehmende,
    unassigned []models.Teilnehmende,
    maxGroupSize int,
    warnings *[]string,
) error {
```

**Step 1 — PreGroup placement (best-fit, not sequential):**

```go
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
        // Prefer same-OV vehicle; secondary: most remaining capacity (fewer TN = better)
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
```

**Step 2 — sort unassigned (unchanged):**
```go
sort.Slice(unassigned, func(i, j int) bool { ... })  // OV → Geschlecht → Alter, unchanged
```

**Step 3 — main distribution loop (collects overflow):**

```go
var overflow []models.Teilnehmende
for _, tn := range unassigned {
    idx := findBestGroupWithCapacity(groups, tn, maxGroupSize)
    if idx < 0 {
        overflow = append(overflow, tn)  // no room; handle below
    } else {
        addTeilnehmendeToGroup(&groups[idx], tn)
    }
}
```

**Step 4 — +1 exception or plain overflow:**

```go
if len(overflow) > 0 {
    // Collect +1-eligible groups: vehicle has physical headroom beyond maxGroupSize.
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
        // +1 exception: vehicle seats beyond maxGroupSize absorb all overflow.
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
        // Cannot fully resolve — place in least-full group and let Phase 5 warn.
        for _, tn := range overflow {
            leastFull := 0
            for i, g := range groups {
                if len(g.Teilnehmende) < len(groups[leastFull].Teilnehmende) {
                    leastFull = i
                }
            }
            addTeilnehmendeToGroup(&groups[leastFull], tn)
        }
        // Phase 5 (checkCapacityWarnings) will emit the per-group overload warning.
    }
}
return nil
```

#### 7.4.5 `findBestGroupWithCapacity` — remove fallback, return -1

```go
func findBestGroupWithCapacity(groups []models.Group, tn models.Teilnehmende, maxGroupSize int) int {
    bestIdx := -1
    bestScore := math.MaxFloat64

    for i, group := range groups {
        cap := effectiveCapacity(group, maxGroupSize)   // use new helper
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

    return bestIdx  // -1 when all groups are at capacity; caller handles overflow
}
```

Note: the old double-check `len(group.Teilnehmende) >= maxGroupSize` is now subsumed by `effectiveCapacity` — `min(maxGroupSize, ...)` already ensures the cap never exceeds `maxGroupSize`.

---

### 7.5 `test/services_test.go`

Each new test follows the same pattern as existing tests: `setupFullTestDB(t)`, insert vehicles/betreuende/TN, call `services.CreateBalancedGroups(db, maxGroupSize, minGroupSize)`, assert on saved groups + returned warning string.

Key helpers needed (all already exist in `test/queries_test.go`):
- `insertFahrzeug(db, bezeichnung, ortsverband, sitzplaetze, fahrerName string)`
- `insertBetreuende(db, name, ortsverband string, fahrerlaubnis bool)`
- `insertTeilnehmende(db, name, ortsverband, geschlecht string, alter int, preGroup string)`

The 12 test functions map directly to §6: one function per row. Tests asserting on Phase 6 messages check `strings.Contains(warning, "ℹ️ Fahrzeuge zu klein")` or `"ℹ️ Nicht benötigte Fahrzeuge"`.

---

### 7.6 Implementation order (safe sequence)

1. `backend/config/config.go` — add `MinGroesse`, update defaults, TOML, Validate
2. `app_handlers.go` + `backend/handlers/files.go` — thread `minGroupSize` through (compile-time check confirms all call sites)
3. `backend/services/distribution.go` — implement in this order to keep it compilable at each step:
   a. Add `effectiveCapacity` helper
   b. Update `CreateBalancedGroups` signature only (add parameter, no body changes yet — pass `minGroupSize` through but don't use it)
   c. Replace `distributeVehicles` / delete `findGroupForVehicle`
   d. Replace `fillParticipants` (new signature + logic)
   e. Update `findBestGroupWithCapacity` (return -1, use `effectiveCapacity`)
   f. Implement Phase 0 filter + Phase 6 in `CreateBalancedGroups`
4. `test/services_test.go` — add new test functions
5. Run all tests: `go test ./...`

---

## 8. Real-World Validation – 2026Liste_Jugend_extrahiert.xlsx

### 8.1 Dataset Summary

| | Count |
|-|-------|
| Teilnehmende | **106** (9 Ortsverbände) |
| Betreuende | **28** (21 licensed / 7 unlicensed) |
| Fahrzeuge | **20** |
| Total people | **134** |
| Total seats | **154** (surplus: +20) |
| TN seats after Betreuende seated | **126** (need 106; surplus: +20) |
| No PreGroups in this dataset | — |

Ortsverband breakdown:

| OV | TN | Betreuende | Vehicles | Seats | TN cap in own vehicles |
|----|-----|-----------|----------|-------|----------------------|
| Bad Säckingen | 17 | 7 | 4 | 29 | 22 ✓ |
| Konstanz | 15 | 3 | 3 | 22 | 19 ✓ |
| Radolfzell | 9 | 3 | 2 | 15 | 12 ✓ |
| Rottweil | 16 | 3 | 3 | 25 | 22 ✓ |
| Schramberg | 9 | 2 | 2 | 15 | 13 ✓ |
| Stockach | 20 | 4 | 2 | 17 | 13 ⚠️ **overflow: 7 TN need seats elsewhere** |
| Trossingen | 9 | 2 | 2 | 15 | 13 ✓ |
| Villingen-Schwenningen | 6 | 2 | 1 | 8 | 6 ✓ |
| Waldshut-Tiengen | 5 | 2 | 1 | 8 | 6 ✓ |

### 8.2 Data Quality Issues Found

| Issue | Detail | Algorithm response |
|-------|--------|-------------------|
| Driver not in Betreuende list | Maik Merz (Trossingen MTW OV) is named as driver but absent from the Betreuende sheet | Warning: "Fahrer nicht gefunden"; vehicle still assigned to group; group gets no driver-as-Betreuende from Phase 1 |
| 7 vehicles with no driver named | MTW OV (Schramberg), MTW OV/TZ/Privat (Rottweil), MTW OV/TZ/MLW V (Konstanz) | No warning — normal; these groups receive Betreuende from Phase 2 distribution |

### 8.3 Algorithm Trace with This Data

**Phase 0:** 20 group shells created (one per vehicle).  
Compared to the old algorithm: `ceil(106 / 8) = 14` groups — **6 fewer**, causing 6 vehicles to be squeezed into already-full groups and creating the exact overloads the plan addresses.

**Phase 1:** 12 drivers resolved → 12 groups start with 1 Betreuende. 1 warning for Maik Merz. 7 groups start with 0 Betreuende.

**Phase 2:** 16 remaining Betreuende distributed.  
- 9 remaining licensed → first-priority fills the 8 groups with 0 licensed coverage; 1 extra licensed goes to the group with fewest licensed  
- 7 unlicensed → follow OV (Stockach ×2, Bad Säckingen ×2, Waldshut-Tiengen ×1, Villingen-Schwenningen ×1) to their OV's group(s)  
- Result: all 20 groups have at least 1 Betreuende ✓

**Phase 3 (global capacity check):** 154 seats ≥ 134 people → no upfront warning ✓

**Phase 4 (fill TN):** Per-vehicle effectiveCap at ~1–2 Betreuende per group:

| OV | Vehicle | Seats | ~Betreuende | effectiveCap (maxGS=8) |
|----|---------|-------|------------|----------------------|
| Bad Säckingen | GKW Infra | 9 | 2 | min(8, 7) = **7** |
| Bad Säckingen | MTW O | 5 | 2 | min(8, 3) = **3** |
| Bad Säckingen | MTW TZ | 8 | 2 | min(8, 6) = **6** |
| Bad Säckingen | Mzkw | 7 | 1 | min(8, 6) = **6** |
| Konstanz | MLW V | 6 | 1 | min(8, 5) = **5** |
| Konstanz | MTW OV | 8 | 1 | min(8, 7) = **7** |
| Konstanz | MTW TZ | 8 | 1 | min(8, 7) = **7** |
| Radolfzell | MTW OV | 8 | 1 | min(8, 7) = **7** |
| Radolfzell | MTW TZ | 7 | 2 | min(8, 5) = **5** |
| Rottweil | MTW OV | 8 | 1 | min(8, 7) = **7** |
| Rottweil | MTW TZ | 9 | 1 | min(8, 8) = **8** |
| Rottweil | Privat | 8 | 1 | min(8, 7) = **7** |
| Schramberg | MLW | 7 | 1 | min(8, 6) = **6** |
| Schramberg | MTW OV | 8 | 1 | min(8, 7) = **7** |
| Stockach | MTW OV | 8 | 2 | min(8, 6) = **6** |
| Stockach | MTW TZ | 9 | 2 | min(8, 7) = **7** |
| Trossingen | MLW | 7 | 1 | min(8, 6) = **6** |
| Trossingen | MTW OV | 8 | 1 | min(8, 7) = **7** |
| Villingen-Schwenningen | MTW OV | 8 | 2 | min(8, 6) = **6** |
| Waldshut-Tiengen | MTW OV | 8 | 2 | min(8, 6) = **6** |
| **Sum** | | | | **≈ 126** |

Total effectiveCap ≈ 126 ≥ 106 TN → **all TN placed, no overflow** ✓

**Stockach overflow resolved naturally:** Stockach's 2 vehicles hold 13 TN max (6+7). Their 20 TN are distributed by diversity scoring — the 7 excess Stockach TN are placed in groups with the most remaining capacity (e.g. Rottweil MTW TZ with 8 cap, Rottweil MTW OV/Privat with 7 cap each). These groups have spare seats precisely because their OV has fewer TN than vehicle capacity.

**Phase 5:** No overloads expected → no warning.

**Phase 6 (unused vehicles):** With 106 TN across 20 groups averaging 5.3 TN/group, no vehicle is expected to be completely empty given the total numbers. The informational message is emitted only if any group ends up at 0 TN after distribution.

### 8.4 Key Insights from Real Data

1. **Vehicle capacity is always the binding constraint here.** The largest vehicle is 9 seats; with ≥1 Betreuende, effectiveCap ≤ 8 = maxGroupSize. The +1 exception is therefore never triggered with maxGroupSize=8 on this dataset. It would only activate if maxGroupSize were lowered to 6 or 7, or if a larger vehicle (e.g. minibus) were added.

2. **Stockach is the only OV that structurally needs cross-OV transport.** They bring 20 TN but only have capacity for 13 in their own vehicles. The algorithm handles this automatically — 7 Stockach TN ride in other OVs' vehicles.

3. **No PreGroups in this data.** The PreGroup placement logic (§3.4) is not exercised. Test cases must cover it explicitly.

4. **The old algorithm would have created 14 groups** and tried to assign 20 vehicles to them, giving ~1.4 vehicles/group on average — an awkward split that breaks the "one vehicle per group" physical reality.

5. **minGroupSize sensitivity on this dataset:**

   | minGroupSize | Excluded vehicles | Groups | Remaining effectiveCap | OK? |
   |---|---|---|---|---|
   | 0 (disabled) | none | 20 | ≈126 | ✓ |
   | 5 | MTW O Bad Säckingen (5 seats) | 19 | ≈122 | ✓ |
   | 6 | MTW O Bad Säckingen (5 seats) + MLW V Konstanz (6 seats) | 18 | ≈117 | ✓ |

   **minGroupSize=5:** Only MTW O (Bad Säckingen, 5 seats) is excluded — `5 − 1 = 4 < 5`. Driver Kim Waßmer re-distributed as Betreuende. Bad Säckingen keeps 3 other vehicles (GKW Infra 9-seat, MTW TZ 8-seat, Mzkw 7-seat). Net result: 19 groups, distribution unaffected.

   **minGroupSize=6:** Two vehicles excluded — MTW O (5 seats) and MLW V Konstanz (6 seats), both with `seats − 1 < 6`. Removed seats: 11; remaining 143 seats for 134 people ✓. Konstanz keeps MTW OV and MTW TZ (8 seats each), sufficient for their 15 TN. No named driver for MLW V so no Betreuende re-distribution needed. Net result: 18 groups, global capacity ≈117 TN seats vs. 106 needed ✓.
