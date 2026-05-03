# Plan: Rethink Betreuende Balance Algorithm

## Problem Statement

Current distribution produces groups with 1 and 3 betreuende despite the
Phase 2b / rebalanceBetreuendeAfterDrivers logic. The root cause is that
**named car drivers are added to groups AFTER `distributeBetreuende` runs**
(Step 7 comes after Step 5). If multiple named drivers land in the same pool
and pool formation concentrates them in one group, `rebalanceBetreuendeAfterDrivers`
is supposed to fix this — but it is constrained:

1. It cannot move a named driver out of their pool (car stays in pool).
2. It prefers unlicensed moves first, which may not exist.
3. It stops at max−min ≤ 1, but the minimum might already be 1 (violating the
   "at least 2" requirement).

**Scenario that breaks balance:**
- 4 groups, 3 cat-A drivers (all OV-Alpha, cars 1+2+3), 4 cat-B drivers, 0 cat-C
- CarGroups pools: Pool-1 = (G1, G2), Pool-2 = (G3, G4)
- `distributeBetreuende` (Step 5): runs on cat-B drivers only → 1 per group → each group has 1
- Phase 3 driver assignment (Step 7): 3 cat-A drivers all in Pool-1 → 2 go to G1 (fewest-bet
  tiebreak wins for first two, OV match for third) → G1 = 3, G2 = 1, G3 = 1, G4 = 1
- `rebalanceBetreuendeAfterDrivers` (Step 8): can move from G1 to G2 (same pool OK),
  but named driver is pinned (car in Pool-1 → driver must stay in Pool-1)
  → At best: G1=2, G2=2, G3=1, G4=1 — still violates "every group ≥ 2"

## Constraints Confirmed by User

| Constraint | Value |
|---|---|
| Mode with problem | CarGroups (cargroups=ja) |
| Minimum betreuende per group | **2** (hard constraint) |
| Balance goal | max−min ≤ 1 **in addition** to min ≥ 2 |
| Driver must stay with their car's pool | Yes — driver stays in same pool |
| Driver can be in any group within pool | Yes — for balance the driver may go to G2 even if car is in G1/G2 shared pool |
| Rebalance may not move driver out of pool | Yes — pool boundary is hard |
| Each group needs at least 1 licensed driver | Yes — group cannot travel without one |

---

## Root-Cause Analysis of Current Code

### Step 5 → Step 7 ordering problem

`distributeBetreuende` operates only on `freeBetreuende` (cat-B + cat-C). Named
drivers (cat-A) are not yet in groups. The "fewest betreuende" criterion in Phase
3 of `assignCarGroups` tries to balance driver placement, but it cannot know how
many cat-B betreuende each group will ultimately have, nor can it guarantee
≥2 total.

### The min-2 constraint is never checked

Neither `distributeBetreuende` nor `rebalanceBetreuendeAfterDrivers` has a
"minimum 2 per group" requirement. Phase 3 checks for "at least 1" (Phase 3 safety
net) but not 2.

### Pool-boundary rebalance is incomplete

`rebalanceBetreuendeAfterDrivers` moves freely across all groups (it picks
global max → global min). But it correctly skips external/pinned vehicle drivers.
It cannot move an unlicensed betreuende from G3 to G1 because G3 has only 1 betreuende
(min is already at the "can't take from here" threshold). The issue is that the
algorithm only fixes max−min, not that the minimum itself might violate ≥2.

---

## Rethought Algorithm

The fundamental fix is to **treat the minimum-2 requirement as the primary
rebalancing objective**, separate from the max−min ≤ 1 secondary objective.
This requires a two-pass rebalance:

### Pass A — Satisfy min ≥ 2 (hard constraint)

1. Find every group with < 2 betreuende ("starved group").
2. For each starved group, find a donor group with > 2 betreuende.
3. Move one betreuende from donor to starved group.
4. Donor selection rules (in priority order):
   a. Donor must be in the same pool as the starved group (pool-boundary hard rule).
   b. If no same-pool donor exists, search cross-pool — but only move an
      **unlicensed** betreuende (moving a driver between pools would leave the
      pool without its assigned driver). Actually: if a driver is moved cross-pool
      their car stays behind — violating the "driver must stay with car pool" rule.
      Therefore: **cross-pool moves are only valid for non-driver betreuende (cat-B/C)**.
   c. Candidate to move: prefer unlicensed (cat-C), then licensed non-external (cat-B).
      Never move a named car driver (cat-A, `IsExternalDriver` or `FahrerName` matched).
   d. Within the donor group, apply OV-co-location preference:
      prefer a person whose OV still has ≥2 remaining in the donor after removal.
5. After each move, re-check the starved group. If it now has ≥2, move to next.
6. Repeat until no group has < 2 or no valid donor exists (emit warning if unfixable).

### Pass B — Minimise spread (max−min ≤ 1)

After all groups have ≥2 betreuende, apply the existing max−min ≤ 1 rebalance,
but **also respect the pool boundary for named drivers**.
- Only non-driver betreuende (cat-B/C) may cross pool boundaries during this pass.
- Named car drivers may only move within their pool.
- Each licensed betreuende must remain in a group that still has ≥1 licensed after removal.

---

## Integration Point: Where to Run the New Passes

### Option A — Run after Step 7 (current location of rebalanceBetreuendeAfterDrivers)

Replace `rebalanceBetreuendeAfterDrivers` with the new two-pass function.
Pros: clean, one function.
Cons: Pass A may need pool information, which is only available from `lastCarGroups`
(a package-level variable).

### Option B — Integrate into assignCarGroups Phase 3

After all named drivers are placed in Phase 3, run the two passes immediately
while `cg` (CarGroup objects) are still in scope. The pool boundary is naturally
available without needing `lastCarGroups`.

**→ Option B is preferred.** It has clear access to pool structure.

---

## Detailed Change Plan

### Change 1 — New function: `rebalancePoolBetreuende`

**File**: `backend/services/distribution_fixgroupsize.go`  
**Replaces**: the end of `assignCarGroups` (after Phase 3 driver assignment)
and the call to `rebalanceBetreuendeAfterDrivers` in `createGroupsFixGroupSize`.

```
func rebalancePoolBetreuende(carGroups []*models.CarGroup, groups []models.Group, groupIdxByID map[int]int)

Pass A: Ensure every group has ≥ 2 betreuende
  while exists group g with len(g.Betreuende) < 2:
    try to find donor with len(donor.Betreuende) > 2:
      priority 1: donor in same pool as g
      priority 2: donor in any pool (cross-pool, unlicensed-only move)
    pick candidate from donor (OV-co-location preference, unlicensed first)
    move candidate; sync back to flat groups slice
    if no donor found: record warning, continue to next starved group

Pass B: Minimise spread (max−min ≤ 1)
  while max(len) - min(len) > 1:
    pick maxGroup (global max) and minGroup (global min)
    find moveable candidate in maxGroup:
      - not a named car driver (not IsExternalDriver)
      - if licensed: group must retain ≥1 licensed after removal
      - if driver (cat-A) being moved: must stay within same pool
        → if maxGroup pool ≠ minGroup pool: skip licensed
    move candidate to minGroup; sync back
    if no candidate: break
```

### Change 2 — Remove `rebalanceBetreuendeAfterDrivers` call from `createGroupsFixGroupSize`

The new `rebalancePoolBetreuende` runs inside `assignCarGroups` and covers the
same ground with the pool-aware logic. The old call in Step 8 must be removed
to avoid double-processing.

### Change 3 — Update `distributeBetreuende` Phase 3 (safety net)

The Phase 3 safety net currently ensures "at least 1 betreuende per group".
This should be updated to "at least 2" to catch the deficit earlier, before
the vehicle assignment step adds more load.

However: `distributeBetreuende` is called before car drivers are added, so
the betreuende count at that point is only the free betreuende (cat-B/C).
Enforcing ≥2 here may not be possible if there are too few betreuende.
**Decision**: Keep Phase 3 at ≥1 (safety net as before) and rely on
`rebalancePoolBetreuende` Pass A for the ≥2 guarantee after drivers are added.
Emit a clear warning if ≥2 cannot be reached.

### Change 4 — Warning for unresolvable ≥2 constraint

When Pass A cannot find any donor (too few betreuende overall), emit:
```
"⚠️ Gruppe %d hat nur %d Betreuende und konnte nicht auf mindestens 2 aufgefüllt werden –
bitte mehr Betreuende importieren (mindestens 2 × Gruppenanzahl = %d)."
```

---

## Named Driver Pool-Pin Logic (How to Implement)

To know which pool a betreuende belongs to, we need a helper:

```go
// poolOf returns the CarGroup that contains the group with groupID.
func poolOf(carGroups []*models.CarGroup, groupID int) *models.CarGroup {
    for _, cg := range carGroups {
        for _, g := range cg.Groups {
            if g.GroupID == groupID {
                return cg
            }
        }
    }
    return nil
}
```

During Pass A and Pass B, before moving a betreuende across pool boundaries:
- Check if `b.Fahrerlaubnis && !b.IsExternalDriver` (category A or B).
- If the betreuende is a named car driver: `poolOf(carGroups, srcGroup) != poolOf(carGroups, dstGroup)` → skip.
- Cat-B (licensed, not a named driver): allowed cross-pool (their car is not in any pool by definition).
- Cat-A (named driver): only within-pool moves allowed.

Identifying cat-A inside the group: after `assignCarGroups` Phase 3, a named
driver's `Betreuende` entry has `IsExternalDriver=true` OR its `Name` matches a
key in `carDrivers`. We can set a new boolean field `IsCarDriver bool` on the
model, OR we can look up by name against `carDrivers` keys.

**→ Use the existing `carDrivers` map** (available in `assignCarGroups` scope):
Build a `namedDriverNames map[string]bool` (lowercased names of all non-external
car drivers) and use it in the rebalance passes.

---

## Open Questions

| Question | Decision |
|---|---|
| Should ≥2 also be enforced in 1:1 mode (cargroups=nein)? | **Yes** — same rebalance applies |
| Should ≥2 be enforced in Klassisch / Fahrzeuge modes? | **Yes** — universal constraint |
| What if total betreuende < 2×numGroups? Can we still guarantee ≥2? | No — emit warning; this is a data constraint the user must fix |
| Should we update the plan doc for Changes A–E after this rethink? | Yes — some of those changes are subsumed by the new algorithm |

---

## Test Cases to Add

| Scenario | Expected outcome |
|---|---|
| 4 groups, 3 cat-A all in same OV, 2 pools (2+2) | After rebalance: each group ≥2, spread ≤1 |
| 5 betreuende, 4 groups (below min) | Warning emitted; best effort: 2,1,1,1 → Pass A fixes to 2,2,1,0 or emits warning when impossible |
| Pool with 2 groups: driver added to G1 making G1=3, G2=1 | Pass A: move 1 betreuende from G1 to G2 (same pool) → 2,2 |
| Cross-pool deficit: G3 has 1 (different pool from where donors are) | Move cat-B/C cross-pool; don't move cat-A |
| All betreuende in one group are named car drivers | Cannot rebalance; warn |

---

## Implementation Order

1. Add `namedDriverNames` helper in `assignCarGroups`.
2. Implement `rebalancePoolBetreuende` (Pass A + Pass B).
3. Call it at the end of `assignCarGroups` (before the capacity check, after Phase 3).
4. Remove the `rebalanceBetreuendeAfterDrivers` call from `createGroupsFixGroupSize` Step 8.
5. Update Phase 4 warning in `distributeBetreuende` to also flag groups with 1 betreuende
   (soft warning, not hard block).
6. Write test cases.
