# Plan: FixGroupSize — Hard Maximum Constraint

**Date:** 2026-05-02  
**Status:** ✅ Implemented

---

## 1. Problem Statement

In `FixGroupSize` mode the number of groups is currently computed as:

```go
numGroups = int(math.Round(float64(N) / float64(fixSize)))
```

`math.Round` rounds to the nearest integer, which rounds **down** whenever the
fractional part is < 0.5. When it rounds down, the leftover participants spill
into a smaller set of groups, causing one or more groups to receive
`fixSize + 1` participants — **violating the configured maximum**.

### Concrete example

| N | fixSize | math.Round result | base | extra | Max group size | Violates? |
|---|---------|-------------------|------|-------|----------------|-----------|
| 105 | 8 | round(13.125) = **13** | 8 | 1 | **9** | ✅ yes |
| 17  | 8 | round(2.125) = **2**  | 8 | 1 | **9** | ✅ yes |
| 25  | 8 | round(3.125) = **3**  | 8 | 1 | **9** | ✅ yes |
| 110 | 8 | round(13.75) = **14** | 7 | 12| 8  | ❌ no  |
| 104 | 8 | round(13.0) = **13**  | 8 | 0 | 8  | ❌ no  |

Violation occurs exactly when `round(N/fixSize)` rounds **down** and
`N % numGroups > 0`. In every such case `base = fixSize` and
`base + 1 = fixSize + 1`.

---

## 2. Root Cause

`math.Round` was used to stay "close to fixgroupsize". But the semantics of
`fixgroupsize` is a **hard upper bound per group**, not a target average.
When rounding down, there are fewer groups than needed to stay within the bound.

---

## 3. Proposed Fix

Replace `math.Round` with ceiling division:

```go
// BEFORE
numGroups := int(math.Round(float64(N) / float64(fixSize)))

// AFTER
numGroups := (N + fixSize - 1) / fixSize   // integer ceiling division, no import needed
```

Or equivalently using `math.Ceil` (already imported):

```go
numGroups := int(math.Ceil(float64(N) / float64(fixSize)))
```

The integer form `(N + fixSize - 1) / fixSize` avoids floating-point and is
slightly more readable.

### Why this works

`ceil(N/fixSize)` guarantees `numGroups × fixSize ≥ N`, so
`base = N/numGroups ≤ fixSize`, and at most `base + 1 ≤ fixSize` (for the
`extra` groups). No group ever exceeds `fixSize`.

### Updated examples

| N | fixSize | ceil result | base | extra | Group sizes | Max |
|---|---------|-------------|------|-------|-------------|-----|
| 105 | 8 | ceil(13.125) = **14** | 7 | 7 | 7 × 8, 7 × 7 | **8** ✓ |
| 17  | 8 | ceil(2.125) = **3**   | 5 | 2 | 2 × 6, 1 × 5 | **6** ✓ |
| 25  | 8 | ceil(3.125) = **4**   | 6 | 1 | 1 × 7, 3 × 6 | **7** ✓ |
| 110 | 8 | ceil(13.75) = **14**  | 7 | 12| 12 × 8, 2 × 7 | **8** ✓ |
| 104 | 8 | ceil(13.0) = **13**   | 8 | 0 | 13 × 8 | **8** ✓ |
| 16  | 8 | ceil(2.0) = **2**     | 8 | 0 | 2 × 8 | **8** ✓ |
| 8   | 8 | ceil(1.0) = **1**     | 8 | 0 | 1 × 8 | **8** ✓ |
| 1   | 8 | ceil(0.125) = **1**   | 1 | 0 | 1 × 1 | **1** ✓ |

---

## 4. Note on the 105-Participant Example

The request mentions "13 groups, some with 7, some with 8". This is
**mathematically impossible**: 13 groups with max 8 per group can only hold
104 participants (13 × 8 = 104 < 105). The correct answer is **14 groups**:

```
7 groups × 8 = 56
7 groups × 7 = 49
Total        = 105 ✓   max per group = 8 ✓
```

> **Question Q1 (below):** Is 14 groups acceptable for 105 participants /
> fixSize=8, or is there a special rule the operator wants?

---

## 5. Impact on Existing Behaviour

### 5.1 Cases that change

Only cases where `math.Round` rounds **down** and there is a remainder are
affected. Specifically when `N mod fixSize > 0` and the fractional part of
`N/fixSize < 0.5`:

| Fraction range | Old (round) | New (ceil) | Δ groups |
|----------------|-------------|------------|----------|
| 0.0 (exact)    | same        | same       | 0 |
| 0.0 < f < 0.5  | rounds down | ceil up    | **+1** |
| f = 0.5        | rounds up*  | ceil up    | 0 |
| 0.5 < f < 1.0  | rounds up   | ceil up    | 0 |

\* Go `math.Round(0.5) = 1` (round half away from zero).

**Summary:** One extra group is created whenever the current code would produce
a group exceeding `fixSize`. No other cases are affected.

### 5.2 CarGroups pool formation

Adding one extra group affects the CarGroups DFS+DP solver. The extra group is
smaller (≤ fixSize), so its seat demand is lower — this is strictly easier to
satisfy. No changes needed in the CarGroups code.

### 5.3 PreGroup validation

`validatePreGroups(teilnehmende, fixSize)` compares each PreGroup's size against
`fixSize`. This stays correct: a PreGroup that would exceed `fixSize` is still
an error regardless of rounding.

---

## 6. Affected Files

| File | Change |
|------|--------|
| `backend/services/distribution_fixgroupsize.go` | Line ~53: replace `math.Round` with integer ceiling |
| `backend/services/distribution_fixgroupsize.go` | If `math` import is only used for `math.Round` → `math.Ceil`, consider switching to integer form to keep the `math` import used only by `math.MaxFloat64` elsewhere |
| `test/services_test.go` | Add `FixGroupSize`-specific tests (see §7) |
| `ai-artefacts/plan-fixgroupsize-distribution.md` | Update §4.1 table to reflect new formula |

---

## 7. Tests to Add

New test function in `test/services_test.go`:

```go
// TestFixGroupSize_NeverExceedsFixSize verifies that no group ever exceeds
// fixgroupsize regardless of participant count.
func TestFixGroupSize_NeverExceedsFixSize(t *testing.T) {
    cases := []struct {
        n        int
        fixSize  int
        wantMax  int
    }{
        {105, 8, 8},
        {17,  8, 8},  // was: 9 with old math.Round
        {25,  8, 8},  // was: 9 with old math.Round
        {104, 8, 8},
        {16,  8, 8},
        {9,   8, 8},  // was: 9 with old math.Round
    }
    ...
}
```

Also update the existing `"25 participants"` entry in
`TestCreateBalancedGroups_NoGroupExceedsMaxSize` — that test already checks the
invariant for the "Klassisch" path; a parallel one for FixGroupSize makes
regressions obvious.

---

## 8. Decisions

| Q | Answer |
|---|--------|
| Q1 — 14 groups for 105/fixSize=8 | ✅ Acceptable — fixSize is a hard maximum |
| Q2 — warn when group < fixSize-2 | ✅ Yes, emit warning |
| Q3 — minimum group size | ✅ `fixSize - 2` (groups below this threshold trigger the warning) |

---

## 9. Implementation Plan (once questions answered)

1. **Change formula** in `distribution_fixgroupsize.go`:
   ```go
   // Replace:
   numGroups := int(math.Round(float64(N) / float64(fixSize)))
   // With:
   numGroups := (N + fixSize - 1) / fixSize
   ```
   Remove `math.Round` call; verify `math` import still needed (it is: `math.MaxFloat64`, `math.Ceil` in DFS solver).

2. **Add tests** for the violation cases listed in §7.

3. **Update plan-fixgroupsize-distribution.md** §4.1 table.

4. **Update changelog** with a [0.1.9] or [0.1.8] patch note.
