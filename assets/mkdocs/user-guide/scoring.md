# Ergebniseingabe

## Stationen verwalten

Bevor Ergebnisse eingetragen werden, können Stationen über **Admin → „Stationen umbenennen"** verwaltet werden.

!!! info "📸 Screenshot: `station-editor-dialog.png`"
    _Station-Editor-Dialog — Stationen hinzufügen, umbenennen, löschen_

Der Dialog bietet folgende Funktionen:

| Funktion | Beschreibung |
|----------|-------------|
| **Umbenennen** | Stationsname inline bearbeiten, dann „Alle speichern" klicken. Geänderte Felder werden orange hervorgehoben. |
| **Hinzufügen** | Neuen Stationsnamen in das Eingabefeld unten eingeben und „+ Hinzufügen" klicken (oder `Enter`). Die neue Station erscheint sofort in der Liste. |
| **Löschen** | „✕"-Schaltfläche neben einer Station klicken. Eine Bestätigung ist erforderlich. |

!!! warning "Gesperrt nach erster Ergebniseingabe"
    Sobald das erste Ergebnis gespeichert wurde, ist der Station-Editor schreibgeschützt. Die Stationsliste wird dann nur noch zur Ansicht angezeigt. Diese Einschränkung verhindert inkonsistente Auswertungen.

---

## Eingabeformular öffnen

**📝 Daten → "Ergebniseingabe"** öffnet die Scoring-Ansicht.

Alternativ aus der **Eingabeübersicht**: Beliebige Zelle anklicken, um direkt zur Eingabe der jeweiligen Gruppe/Station zu springen.

## Ergebnisse eintragen

1. **Gruppe** aus dem Dropdown auswählen.
2. Die Stationstabelle lädt alle Stationen — bereits gespeicherte Ergebnisse sind vorausgefüllt.
3. Punktzahl je Station eingeben (gültiger Bereich: `min_punkte`–`max_punkte`, Standard **100–1200**).
4. **"Speichern"** klicken (oder die Einzelspeicher-Schaltfläche pro Station).

!!! info "📸 Screenshot: `scoring-form.png`"
    _Ergebniseingabe — Gruppenauswahl und Stationstabelle mit Punktfeldern_

!!! info "Dirty Tracking"
    Nicht gespeicherte Änderungen werden verfolgt. Beim Gruppenwechsel mit ungespeicherten Änderungen erscheint ein Modal zum Speichern oder Verwerfen.

## Validierung

Alle Ergebnisse werden im Backend validiert:

- Werte außerhalb `[min_punkte, max_punkte]` werden abgelehnt.
- Beide Grenzen kommen aus `config.toml` — Änderungen wirken sofort ohne Neustart.

| Einstellung | Standard | Beschreibung |
|-------------|----------|--------------|
| `min_punkte` | 100 | Mindestpunktzahl pro Station |
| `max_punkte` | 1200 | Höchstpunktzahl pro Station |

Siehe [Konfiguration](../developer/configuration.md).

## Massenspeicherung

Das Formular erlaubt das Speichern aller Stationen auf einmal. Jeder Speichervorgang nutzt `INSERT OR REPLACE` — Duplikate sind durch die `UNIQUE(group_id, station_id)`-Datenbank-Constraint ausgeschlossen.
