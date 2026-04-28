# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.4] - 2026-04-27

### Added
- Vehicle-first group distribution algorithm: when vehicles are present, groups are formed one-per-vehicle instead of by participant count alone.
  - `min_groesse` config option (default 6): vehicles with fewer than `min_groesse` passenger seats are excluded; too-small or surplus vehicles are reported as informational warnings.
  - `max_groesse` cap is applied per group using effective capacity (`vehicle seats − Betreuende count`) so that non-driver Betreuende do not silently reduce the number of available TN slots.
- Phase ordering: drivers are assigned first, then Teilnehmende (using full seat capacity), then additional Betreuende — preventing non-driver Betreuende from consuming TN seats.
- Overload relief (Phase 3c): after all people are placed, any group whose headcount exceeds its vehicle seat count has a Teilnehmende (or non-driver Betreuende as fallback) moved to the group with the most spare seats. Drivers are never moved.
- Betreuende:TN ratio rebalancing (Phase 3d): a non-driver Betreuende is swapped with a Teilnehmende between the group with the highest and the group with the lowest Betreuende:TN ratio, repeating until no swap reduces the maximum ratio further. Total headcount per group is preserved, so seat capacity is unaffected.
- Fallback driver assignment (Phase 3b): if a vehicle in a group has no resolved driver, the group's first licensed Betreuende is assigned as the vehicle's driver name.
- `SaveGroupFahrzeuge` now persists any `FahrerName` changes made during distribution back to the `fahrzeuge` table, so fallback driver names are visible in generated PDFs.

### Fixed
- Groups smaller than `min_groesse` caused by too many vehicles: `numGroups` is now capped to `⌊TN count / min_groesse⌋`; excess eligible vehicles are reported as unused.

## [0.1.3] - 2026-04-14

### Added
- Admin: "Master-Excel umwandeln" button — converts a proprietary source Excel into the format required by "Excel einlesen". A dialog asks the user to choose the event type (Jugend or Mini) before the file picker opens.
  - Jugend: reads the `Teilnehmende` sheet (separating JuHe participants and supervisors) and the `Fahrzeuge` sheet; maps `Fahrer` column to the output vehicle list.
  - Mini: vehicle data omitted; the output `Fahrzeuge` sheet is written with headers only.
  - Source `Fahrerlaubnis` column: any non-empty value other than `"/"` is treated as a valid licence.
- Admin: "Namen korrigieren" button — two-step dialog to fix name typos in the live database. Select an Ortsverband from a dropdown, edit names inline per row, save only changed entries. Per-row success/error indicators are shown after saving.

### Fixed
- Certificates: participants without recorded evaluation data previously showed "Platz 0" — they now correctly display "Teilnahme".
- Certificates: rank display is now consistent for all values — every positive rank uses the `"X. Platz"` format (previously ranks 1–3 used a different code path).

## [0.1.2] - 2026-03-23

### Changed
- Renamed all occurrences of "Teilnehmer" to "Teilnehmende" throughout — display text, button labels, code identifiers, Go structs, and function names
- Database table `teilnehmer` renamed to `teilnehmende`; all SQL queries and FK references updated accordingly
- Excel import sheet name changed from `Teilnehmer` to `Teilnehmende`
- Button label "Teilnehmer zu Gruppen" renamed to "Gruppen zusammenstellen"
- Ergebniseingabe mode now collapses all three button-area columns for a focused score-entry layout

## [0.1.1] - 2026-03-13

### Changed
- Consolidated redundant `rel_tn_grp` table into `gruppe` — all queries, inserts, and indexes now use `gruppe` exclusively
- `gruppe` table definition tightened: `group_id` and `teilnehmer_id` are now `NOT NULL`, `teilnehmer_id` has a `UNIQUE` constraint, and the foreign key correctly references `teilnehmer(teilnehmer_id)`
- Participant certificates: "Jugendolympiade" heading moved 1.5cm lower; gap between heading and year reduced
- Participant certificates: rank text enlarged (size 22, bold) and highlighted in gold for better visibility
- Participant certificates: spacing between rank and group members table reduced

### Removed
- `rel_tn_grp` table (duplicate of `gruppe`); double-write on group save eliminated

### Fixed
- Foreign key on `gruppe.teilnehmer_id` previously referenced the wrong column (`teilnehmer.id` instead of `teilnehmer.teilnehmer_id`)
- Foreign key enforcement was never active — `PRAGMA foreign_keys = ON` is now set on every database connection (initial open and after restore)
- `teilnehmer.teilnehmer_id` missing `UNIQUE` constraint caused FK mismatch error on Excel reload with FK enforcement enabled
- Invalid FK on `group_station_scores.group_id` referencing non-unique `gruppe(group_id)` removed; integrity maintained at application level
- Ortsverband evaluation Teilnehmer count inflated by number of stations — fixed with `COUNT(DISTINCT teilnehmer_id)`
- Participant certificates: "Gruppenmitglieder" label had stray colon and was left-aligned instead of centered
- Participant certificates: table rows drifted left of content area — `SetX(contentLeft)` now applied per row
- Participant certificates: content area left margin adjusted from 5mm to 10mm

## [0.1.0] - 2026-03-13

### Added
- Initial release of Jugendolympiade Verwaltung
- Group management (Gruppen) with tabbed view
- Results entry (Ergebniseingabe) with group-first workflow — select a group to view all stations in a table with individual score inputs (100–1200)
- Individual and bulk save for station scores; existing scores pre-populated on load
- Quick navigation button in Gruppen view to jump directly to Ergebniseingabe for a selected group
- Group evaluation (Gruppenwertung) and Ortsverband evaluation views
- PDF generation for group evaluation, Ortsverband evaluation, and certificates
- Database backup functionality
- Database restore from backup with newest-first sorted list
- Frontend structured by feature (shared, admin, groups, stations, evaluations, reports)
