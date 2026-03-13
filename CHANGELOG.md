# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
