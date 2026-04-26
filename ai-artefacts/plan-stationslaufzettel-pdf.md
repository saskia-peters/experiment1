# Plan: Enhance GruppenPDF Button — Add Station Recording Sheets PDF

**Date:** 2026-04-26

## TL;DR

Extend the single "Gruppen-PDF erstellen" button so it generates **two PDFs** in one click:
1. **Existing**: `Gruppeneinteilung.pdf` (unchanged)
2. **New**: `Stationslaufzettel.pdf` — one sheet per station, listing all groups in a table with blank "Ergebnis" and "Uhrzeit" columns for manual recording during the event.

Group rows show both the numeric ID and the configured name: `"Gruppe 1 — Stangenschlangenbohrer"`.

---

## Phase 1 — Database queries

**File:** `backend/database/queries.go`

Add `GetGroupIDsOrdered(db *sql.DB) ([]int, error)`:
- Query: `SELECT DISTINCT group_id FROM gruppe ORDER BY group_id`
- Returns a plain `[]int` slice of group IDs in ascending order — lightweight, no join needed.

Add `GetStationNamesOrdered(db *sql.DB) ([]models.Station, error)`:
- Query: `SELECT station_id, station_name FROM stations ORDER BY station_name`
- Returns `[]models.Station` (reuse existing struct, `GroupScores` field left empty).
- Minimal query — no score join needed for this use case.

---

## Phase 2 — New PDF generator

**New file:** `backend/io/pdf_station_sheets.go`

Function signature:
```go
func GenerateStationSheetsPDF(db *sql.DB, eventName string, eventYear int, groupNames []string) error
```

Layout (A4 Portrait, same margins as other PDFs):
- **Page header**: Event name + year (same style as `pdf_groups.go`), then station name as large title, sub-label "Stationslaufzettel"
- **Table columns**: "Gruppe" (~100mm) | "Ergebnis" (~45mm) | "Uhrzeit" (~35mm)
- **Table header row**: colored fill using `theme.ColorTableHeader`
- **Group rows**: one row per group, row height 12mm (enough for handwriting), alternating shading, all cells bordered. "Gruppe" cell text: `"Gruppe N — ConfiguredName"` (falls back to `"Gruppe N"` using `config.GetGroupName`)
- **Page overflow handling**:
  - `pdf.SetAutoPageBreak(true, 15)` so rows that don't fit automatically continue on the next page
  - Register `pdf.SetHeaderFunc(...)` that re-renders the station name header and table column headers on every continuation page, with `"— Fortsetzung"` appended to the station name — ensures every printed sheet is standalone-usable
- Uses `DefaultTheme` and `enc()` helper — same pattern as `pdf_groups.go`
- Saves to `filepath.Join(pdfOutputDir, "Stationslaufzettel.pdf")`
- Returns an error if no stations are found

---

## Phase 3 — Update backend handler

**File:** `backend/handlers/reports.go`

Update signature: `GeneratePDF(db *sql.DB, eventName string, eventYear int, groupNames []string) map[string]interface{}`

Changes:
- Add `groupNames []string` parameter and pass it through to `io.GenerateStationSheetsPDF`
- After the existing `io.GeneratePDFReport()` call, call `io.GenerateStationSheetsPDF(db, eventName, eventYear, groupNames)`
- **Fail hard** if either PDF fails — return an error response that identifies which PDF failed
- On success, add `"file2"` and `"path2"` keys (pointing to `Stationslaufzettel.pdf`) to the returned map

---

## Phase 4 — Update Wails binding

**File:** `app_handlers.go` (around line 135)

Pass configured group names to the handler:
```go
func (a *App) GeneratePDF() map[string]interface{} {
    return handlers.GeneratePDF(a.db, a.cfg.Veranstaltung.Name, a.cfg.Veranstaltung.Jahr, a.cfg.Gruppen.Gruppennamen)
}
```

---

## Phase 5 — Update frontend

**File:** `frontend/reports/pdf-handlers.js`

Update `handleGeneratePDF()`:
- Loading message: `"PDFs werden erstellt..."`
- Success message: `"✅ Gruppen-PDF und Stationslaufzettel erfolgreich erstellt!"`

---

## Files touched

| File | Change |
|---|---|
| `backend/database/queries.go` | Add `GetGroupIDsOrdered` and `GetStationNamesOrdered` |
| `backend/io/pdf_station_sheets.go` | **New file** — `GenerateStationSheetsPDF` |
| `backend/io/pdf_groups.go` | Reference only (layout patterns: `enc`, `DefaultTheme`, `ensurePDFDirectory`) |
| `backend/handlers/reports.go` | Update `GeneratePDF` signature, add second PDF call |
| `app_handlers.go` | Update Wails binding to pass `groupNames` |
| `frontend/reports/pdf-handlers.js` | Update status messages |

---

## Verification checklist

1. `go build ./...` — no compile errors
2. Load an Excel file in the app, click "Gruppen-PDF erstellen"
3. `pdfdocs/Gruppeneinteilung.pdf` still generates correctly (unchanged)
4. `pdfdocs/Stationslaufzettel.pdf` is created
5. Station sheets PDF: one section per station, all groups listed, "Ergebnis" + "Uhrzeit" columns blank and bordered
6. Group rows show `"Gruppe N — Name"` format
7. Stations with many groups: overflow onto continuation pages with repeated station header and column headers (`"— Fortsetzung"`)
8. Error case: no data loaded → graceful error message indicating which PDF failed

---

## Decisions

| Decision | Choice |
|---|---|
| Button behaviour | One click generates both PDFs atomically |
| Failure policy | Fail hard — error message names the failing PDF |
| Group label | `"Gruppe N — ConfiguredName"` via `config.GetGroupName()` |
| Time column | Single "Uhrzeit" column (not split into Ankunft/Abfahrt) |
| Page size | Portrait A4, consistent with all other PDFs |
| Station order | Alphabetical by `station_name` |
| Overflow | `SetAutoPageBreak` + repeated header via `SetHeaderFunc` |
