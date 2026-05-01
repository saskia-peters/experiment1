# Plan: Betreuende Distribution Improvements

## Context

Two separate improvements to the Betreuende distribution logic, triggered by user observations:

1. **OV co-location**: Betreuende from the same Ortsverband (OV) should stay together in
   the same group, as long as this does not result in imbalanced group counts.
2. **OV-fair driver round-robin**: When assigning licensed Betreuende (drivers) to groups,
   cycle through OVs (most-drivers-first) instead of processing all drivers from one OV
   before moving to the next. This prevents one OV from supplying all drivers while
   another OV contributes none.

**Scope**: FixGroupSize + Fahrzeuge modes. Klassisch (no-vehicle) excluded.  
Since `distributeBetreuende` is shared code, its improvements naturally apply everywhere — 
this is acceptable (it is a pure quality improvement, not a mode-specific change).

---

## Current Behaviour Analysis

### `distributeBetreuende` (shared, `distribution.go:822`)

| Phase | What happens | OV-awareness |
|-------|-------------|--------------|
| Phase 1 | Licensed sorted by OV alphabetically, then assigned one-per-group via `findGroupForLicensed` | Partial — tiebreak only |
| Phase 2 | Unlicensed → group with same-OV licensed, else same-OV any, else fewest-Betreuende | Yes |
| Phase 2b | Rebalance: move unlicensed from max-group to min-group | ❌ None |
| Phase 3 | Emergency: take Betreuende from richest group to fill empty group | ❌ None |

**Phase 1 problem example**: 3 drivers from OV-Alpha, 1 from OV-Beta, 4 groups.
- Current order fed into `findGroupForLicensed`: Alpha1, Alpha2, Alpha3, Beta1
- Result: Group 1 = Alpha, Group 2 = Alpha, Group 3 = Alpha, Group 4 = Beta
- With round-robin: Group 1 = Alpha1, Group 2 = Beta1, Group 3 = Alpha2, Group 4 = Alpha3

**Phase 2b problem example**: OV-Alpha has 2 Betreuende (both in Group 1, count=3).
OV-Beta has 1 Betreuende (Group 2, count=1). Phase 2b moves one Alpha person to Group 2 —
correct balance, but it splits the Alpha pair. OV-aware: could instead move a Betreuende 
whose OV is already represented in the destination.

### Driver fallback (per mode)

| Function | Empty FahrerName | Not-found FahrerName |
|----------|-----------------|----------------------|
| `distributeVehicles` (Fahrzeuge mode) | Skip silently | Warn; Phase 3b later assigns first licensed Betreuende ✅ |
| `assignVehiclesOneToOne` (FixGroupSize, `cargroups=nein`) | Skip silently ❌ | Warn only ❌ |
| `assignCarGroups` (FixGroupSize, `cargroups=ja`) | Skip silently ❌ | Warn + fallback from CarGroup pool ✅ |

Fahrzeuge mode is already handled via Phase 3b in `createGroupsFahrzeuge`.

---

## Planned Changes

### Change A — OV round-robin for licensed Betreuende placement

**File**: `backend/services/distribution.go`  
**Location**: Start of `distributeBetreuende`, before Phase 1 loop.

**New helper**: `ovRoundRobinOrder(licensed []models.Betreuende) []models.Betreuende`

```
1. Group licensed by OV → map[ov][]Betreuende
2. Collect OVs, sort by driver count descending (most drivers → leads each round)
   Tiebreak: alphabetical OV name (deterministic)
3. Interleave: for round r=0,1,2,...
     for each OV in sorted order:
         if OV has a driver at index r → append to result
   until all drivers placed
4. Return interleaved slice
```

**Example** (OV-Alpha 3 drivers, OV-Beta 1, OV-Gamma 2):

| Round | OV-Alpha (3) | OV-Gamma (2) | OV-Beta (1) |
|-------|-------------|-------------|------------|
| 1     | Alpha1      | Gamma1      | Beta1      |
| 2     | Alpha2      | Gamma2      | —          |
| 3     | Alpha3      | —           | —          |

Result fed to `findGroupForLicensed` (unchanged) → each round fills one slot per group.

**Change**: Replace the two `sort.Slice` calls on `licensed`/`unlicensed` at lines ~838-848
with a call to the new helper for `licensed` only. The `unlicensed` sort stays.

---

### Change B — OV-aware source selection in Phase 2b

**File**: `backend/services/distribution.go`  
**Location**: Phase 2b rebalancing loop (~line 888).

**Current**: picks first unlicensed Betreuende from the max-group.

**New logic for "who to move"**:
```
candidates = all unlicensed Betreuende in max-group
preferred  = candidates where OV count in max-group >= 2 
             (moving leaves OV still represented in max-group)
pick       = first of preferred; if none → first of candidates (current fallback)
```

**New logic for "where to move"** (destination):
```
all_min = all groups with len(Betreuende) == minCount
preferred_dest = first group in all_min that already has a Betreuende from same OV as moved person
dest = preferred_dest if found, else minIdx (current behavior)
```

This preserves the balance invariant (max−min ≤ 1 threshold unchanged) while preferring
moves that do not break OV clusters.

---

### Change C — OV-aware source selection in Phase 3

**File**: `backend/services/distribution.go`  
**Location**: Phase 3 loop (~line 916).

**Current**: prefers unlicensed from donor, falls back to licensed.

**New logic for "who to move from donor"**:
```
candidates = all unlicensed Betreuende in donor-group
preferred  = candidates where OV count in donor-group >= 2
pick       = first of preferred; if none → first of candidates; then fall back to licensed (current)
```

The destination is always the empty group — no change needed there.

---

### Change D — Missing driver fallback in `assignVehiclesOneToOne`

**File**: `backend/services/distribution_fixgroupsize.go`  
**Location**: `assignVehiclesOneToOne`, driver resolution block.

**Current**:
```go
if car.FahrerName == "" {
    continue   // no fallback
}
// checks driverMap, warns if not found
```

**New**:
```go
driverResolved := false
if car.FahrerName != "" {
    // existing driverMap lookup + warn if not found
    driverResolved = found
}
if !driverResolved {
    // assign first licensed Betreuende from the group as fallback driver
    for _, b := range groups[gi].Betreuende {
        if b.Fahrerlaubnis {
            groups[gi].Fahrzeuge[last].FahrerName = b.Name
            break
        }
    }
}
```

Note: `distributeBetreuende` runs before `assignVehiclesOneToOne` (Steps 5→7 in
`createGroupsFixGroupSize`), so the group's Betreuende are populated when this runs.

---

### Change E — Missing driver fallback in `assignCarGroups` (empty FahrerName)

**File**: `backend/services/distribution_fixgroupsize.go`  
**Location**: driver resolution loop after Phase 2, currently:

```go
if car.FahrerName != "" {
    // check driverMap, warn + Phase 3b fallback for not-found
}
```

**New**: extend the condition so empty FahrerName also triggers the fallback:

```go
needsDriver := car.FahrerName == ""
if !needsDriver {
    k := driverKey{...}
    if _, found := driverMap[k]; !found {
        // warn
        needsDriver = true
    }
}
if needsDriver {
    // Phase 3b: assign first licensed Betreuende from any group in CarGroup pool
    for _, g := range cg.Groups {
        for _, b := range g.Betreuende {
            if b.Fahrerlaubnis {
                car.FahrerName = b.Name
                goto nextCar
            }
        }
    }
}
```

---

## Impact Summary

| Change | File | Function | Lines (approx) |
|--------|------|----------|----------------|
| A — OV round-robin | `distribution.go` | `distributeBetreuende` + new helper `ovRoundRobinOrder` | ~838–870 |
| B — OV-aware Phase 2b | `distribution.go` | `distributeBetreuende` | ~888–910 |
| C — OV-aware Phase 3 | `distribution.go` | `distributeBetreuende` | ~916–960 |
| D — Driver fallback 1:1 | `distribution_fixgroupsize.go` | `assignVehiclesOneToOne` | ~255–280 |
| E — Driver fallback CarGroups | `distribution_fixgroupsize.go` | `assignCarGroups` | ~402–430 |

No changes to:
- Database schema or queries
- Models
- Frontend / Wails bindings
- `distributeVehicles` / `createGroupsFahrzeuge` (Phase 3b already handles missing drivers)
- PDF generators

---

## Test Considerations

All existing tests in `test/services_test.go` and `test/distribution_test.go` must continue
to pass. New test cases to add:

| Scenario | Expected outcome |
|----------|-----------------|
| OV-Alpha 3 drivers, OV-Beta 1 driver, 4 groups | Each of the 4 groups gets exactly one driver; Beta driver not in last group |
| 2 Betreuende from OV-Alpha in one group, Phase 2b triggered | Alpha pair is not split if an alternative person (different OV) is available to move |
| Car with empty FahrerName, group has licensed Betreuende | Car gets fallback driver assigned |
| CarGroup car with empty FahrerName | Car gets first licensed Betreuende from pool |

---

## Open Questions / Decisions Already Made

| Question | Decision |
|----------|----------|
| OV co-location vs. balance — which wins? | Balance wins (max−min ≤ 1); OV co-location is the tiebreaker |
| Round-robin OV order | Most-drivers-first per round |
| Does round-robin also control named car driver? | No — car FahrerName from Excel stays as-is |
| Missing driver when FahrerName empty or not-found | Assign first licensed Betreuende in same group/pool |
| Which modes get the improvements? | FixGroupSize + Fahrzeuge; Klassisch excluded by user (shared code improves it incidentally) |
