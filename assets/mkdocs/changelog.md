# Changelog

Alle wesentlichen Änderungen an diesem Projekt sind hier dokumentiert.

Das Format basiert auf [Keep a Changelog](https://keepachangelog.com/en/1.0.0/), das Projekt folgt [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

---

## [0.1.7] — 2026-05-01

### Hinzugefügt

- **Admin → „Stationen umbenennen"**: Neuer Dialog zum Verwalten der Stationsliste direkt in der Anwendung — ohne erneuten Excel-Import.
    - **Umbenennen**: Stationsnamen inline bearbeiten und mit „Alle speichern" übernehmen. Geänderte Felder werden orange hervorgehoben; je Zeile erscheint nach dem Speichern ein ✔ oder ✖.
    - **Hinzufügen**: Neuen Stationsnamen eingeben und „+ Hinzufügen" klicken (oder `Enter`). Die Station erscheint sofort in der Liste.
    - **Löschen**: „✕"-Schaltfläche neben einer Station klicken; Bestätigung erforderlich.
    - **Schreibschutz**: Sobald das erste Ergebnis gespeichert wurde, ist der Dialog schreibgeschützt (nur Ansicht). Eine Warnmeldung erklärt die Einschränkung.
- **Standard-Stationen beim Import**: Fehlt das Tabellenblatt `Stationen` in der XLSX-Datei oder ist es leer, wird der Import nicht mehr abgelehnt. Stattdessen werden acht Standard-Stationen geladen (`Kübelspritze Zielschiessen`, `Magnetlabyrinth`, `Merkfähigkeit`, `Geräuschlabyrinth`, `Rittersport`, `Koordiniertes Nageln`, `Wurfknoten werfen`, `Polarexpedition`) und eine Warnmeldung angezeigt.
- **Master-Excel Konvertierung — Spalte `Geschlecht`**: Die Quell-Excel-Datei darf nun eine Spalte `Geschlecht` (Werte `m`/`w`/`d`) enthalten. Der Wert wird direkt in das Ziel-XLSX übernommen. Fehlt die Spalte, bleibt `Geschlecht` leer (kein Fehler).
- **OV-Zuteilung PDF** (`OV-Zuteilung.pdf`): Neues PDF, das mit dem bestehenden „Gruppen-PDF erstellen"-Button erzeugt wird. Enthält eine Seite pro Ortsverband mit zwei Tabellen — Betreuende (mit Gruppe, Fahrzeug und Fahrerkennzeichnung) und Teilnehmende (mit Gruppe). Langer Text wird automatisch kleiner dargestellt.
- **Teilnehmende-Karten PDF** (`Teilnehmende-Karten.pdf`): Neues PDF, ebenfalls über „Gruppen-PDF erstellen" erzeugt. A4-Querformat mit je vier A6-Karten (2 × 2) zum Ausschneiden. Jede Karte zeigt Name, Ortsverband und Gruppe (`Gruppe N - Gruppenname`). Karten sind nach OV und Name sortiert.

### Geändert

- Excel-Import: Das Tabellenblatt `Stationen` ist jetzt **optional** (vorher Pflicht).

---

## [0.1.6] — 2026-04-28

### Hinzugefügt

- **Fahrzeug-zuerst-Algorithmus** für die Gruppenverteilung: Wenn Fahrzeuge importiert wurden, erhält jede Gruppe exakt ein Fahrzeug (1:1-Zuweisung sortiert nach OV → Bezeichnung). Gruppenanzahl richtet sich nach der Anzahl der geeigneten Fahrzeuge, nicht nach `max_groesse`.
- **Phase 3c — Überlastungsausgleich**: Gruppen, die nach der TN-Verteilung mehr Personen als Sitzplätze haben, geben automatisch Teilnehmende (oder nicht-fahrende Betreuende) an Gruppen mit Reserveplätzen ab. Läuft still im Hintergrund.
- **Phase 3d — Betreuenden:TN-Verhältnis-Ausgleich**: Tauscht eine Betreuende ↔ Teilnehmende zwischen der Gruppe mit dem höchsten und dem niedrigsten B:TN-Verhältnis, solange der Tausch eine Verbesserung bringt. Gesamtgröße der Gruppen bleibt konstant. Läuft still im Hintergrund.
- **Phase 3b — Automatischer Fahrer-Fallback**: Wurde im Fahrzeug-Blatt kein Fahrer eingetragen oder ist der Name nicht in der Betreuenden-Liste, wird die erste lizenzierte Betreuende der Gruppe automatisch als Fahrerin gesetzt.
- **`min_groesse`-Konfiguration** (`config.toml`, Abschnitt `[gruppen]`): Fahrzeuge, deren Passagierplätze (Sitzplätze − 1) kleiner als `min_groesse` sind, werden beim Verteilen ausgeschlossen. Standard: `6`.
- **Kapazitätsprüfung** (Phase 4): Nach der Verteilung werden Warnungen ausgegeben, wenn die Gesamtzahl der Personen die Gesamtsitzplätze überschreitet oder eine einzelne Gruppe mehr Mitglieder als Sitze hat.

### Behoben

- **PDF-Fahrername fehlte**: `SaveGroupFahrzeuge` schrieb nur die Gruppe↔Fahrzeug-Verknüpfung, aber nicht den aktualisierten `FahrerName` in die `fahrzeuge`-Tabelle. Die Datenbank wird jetzt in derselben Transaktion aktualisiert, sodass der korrekte Fahrername auf den PDFs erscheint.

### Geändert

- Umverteilungsmeldungen für Phase 3c und 3d werden nicht mehr als Benutzerwarnung angezeigt (laufen still).

---

## [0.1.5] — 2026-04-20

### Hinzugefügt

- Gruppen: Benutzerdefinierte Gruppennamen über `gruppen.gruppennamen` in `config.toml` — werden in der Gruppen-Ansicht, der Ergebniseingabe, der Auswertung und auf Teilnehmer-Urkunden angezeigt. Sind weniger Namen als Gruppen eingetragen, wird „Gruppe N" als Fallback verwendet.

### Geändert

- Urkunden-Editor: Standardwert für `urkunden_stil` auf `picture` gesetzt.
- Urkunden-Editor: Element-Tausch-Logik verbessert; Layoutelement-Karten optisch überarbeitet.
- justfile: Build-Ziel `build-win` (ohne Konsolenfenster) entfernt.

---

## [0.1.4] — 2026-04-14

### Hinzugefügt

- Gruppen-Ansicht: Fahrzeug-Abschnitt wird jetzt **immer** angezeigt; bei Gruppen ohne zugewiesenes Fahrzeug erscheint der Hinweis **„Kein Fahrzeug!"** (rot, fett, zentriert).
- justfile: Neues Build-Ziel `build-win` — erstellt ein Windows-Binary ohne Konsolenfenster (`-H windowsgui`).

### Geändert

- Excel-Import: Das Tabellenblatt `Stationen` ist jetzt **Pflicht**. Fehlt das Blatt oder enthält es keine Einträge, wird der Import abgelehnt und die Fehlermeldung „Keine Stationen vorhanden, bitte im XLSX einfügen." angezeigt — bevor die Datenbank verändert wird.
- Excel-Import: Bei einem Ladefehler erscheint zusätzlich ein Modal-Fenster mit dem genauen Fehlertext.
- Dokumentation: Spalte `Geschlecht` im Blatt `Teilnehmende` als manuell ausfüllbar gekennzeichnet (wird nicht automatisch aus dem Anmeldesystem übernommen).

### Behoben

- Gruppenverteilung (Fahrzeug-Pfad): Der Fahrer eines Fahrzeugs wurde nach der Fahrzeugzuweisung aus der Betreuenden-Liste entfernt. Grund war ein Reset der Betreuenden-Liste zu Beginn von `distributeBetreuende`. Der Fahrer erscheint nun korrekt in der Betreuenden-Sektion der Gruppe und wird bei der Sitzplatz-Prüfung genau einmal gezählt.

---

## [0.1.3] — 2026-04-14

### Hinzugefügt

- Admin: Schaltfläche „Master-Excel umwandeln" — wandelt eine Quell-Excel-Datei im internen Format in das Format um, das von „Excel einlesen" erwartet wird. Vor der Dateiauswahl erscheint ein Dialog zur Wahl des Veranstaltungstyps (Jugend oder Mini).
  - Jugend: liest Tabellenblatt `Teilnehmende` (trennt JuHe-Teilnehmende von Betreuenden) und Tabellenblatt `Fahrzeuge`; bildet Spalte `Fahrer` auf die Ausgabe-Fahrzeugliste ab.
  - Mini: Fahrzeugdaten werden nicht übernommen; das Ausgabe-Tabellenblatt `Fahrzeuge` enthält nur die Kopfzeile.
  - Quellspalte `Fahrerlaubnis`: jeder nicht-leere Wert außer `„/"` gilt als gültige Fahrerlaubnis.
- Admin: Schaltfläche „Namen korrigieren" — zweistufiger Dialog zum Korrigieren von Schreibfehlern in Namen direkt in der Datenbank. Ortsverband aus der Dropdown-Liste wählen, Namen zeilenweise bearbeiten, nur geänderte Einträge speichern. Nach dem Speichern wird je Zeile ein Erfolgs- oder Fehlersymbol angezeigt.

### Behoben

- Urkunden: Teilnehmende ohne Auswertungsdaten erhielten bisher fälschlicherweise die Angabe „Platz 0" — korrekt ist jetzt „Teilnahme".
- Urkunden: Platzierungsanzeige ist nun einheitlich — alle positiven Ränge verwenden das Format `„X. Platz"` (zuvor gab es für Ränge 1–3 einen abweichenden Code-Pfad).

---

## [0.1.2] — 2026-03-23

### Geändert

- Alle Vorkommen von „Teilnehmer" in „Teilnehmende" umbenannt — Anzeigetexte, Schaltflächenbeschriftungen, Code-Bezeichner, Go-Structs und Funktionsnamen.
- Datenbanktabelle `teilnehmer` in `teilnehmende` umbenannt; alle SQL-Abfragen und FK-Referenzen aktualisiert.
- Excel-Import-Blattname von `Teilnehmer` auf `Teilnehmende` geändert.
- Schaltflächenbeschriftung „Teilnehmer zu Gruppen" in „Gruppen zusammenstellen" umbenannt.
- Ergebniseingabe-Modus blendet jetzt alle drei Button-Bereichsspalten aus.

---

## [0.1.1] — 2026-03-13

### Geändert

- Redundante `rel_tn_grp`-Tabelle in `gruppe` konsolidiert — alle Abfragen, Inserts und Indizes verwenden ausschließlich `gruppe`.
- `gruppe`-Tabellendefinition verschärft: `group_id` und `teilnehmer_id` sind jetzt `NOT NULL`; `teilnehmer_id` hat einen `UNIQUE`-Constraint; der Foreign Key referenziert korrekt `teilnehmer(teilnehmer_id)`.
- Teilnehmer-Urkunden: „Jugendolympiade"-Überschrift 1,5 cm tiefer; Abstand zwischen Überschrift und Jahr reduziert.
- Teilnehmer-Urkunden: Platzierungstext vergrößert (Größe 22, fett) und gold hervorgehoben.
- Teilnehmer-Urkunden: Abstand zwischen Platzierung und Mitgliedertabelle reduziert.

### Entfernt

- `rel_tn_grp`-Tabelle (Duplikat von `gruppe`); doppeltes Schreiben beim Gruppen-Speichern eliminiert.

### Behoben

- Foreign Key auf `gruppe.teilnehmer_id` referenzierte die falsche Spalte (`teilnehmer.id` statt `teilnehmer.teilnehmer_id`).
- FK-Enforcement war nie aktiv — `PRAGMA foreign_keys = ON` wird jetzt bei jeder Datenbankverbindung gesetzt.
- `teilnehmer.teilnehmer_id` fehlte `UNIQUE`-Constraint, was FK-Fehler beim Excel-Reload verursachte.
- Ungültiger FK auf `group_station_scores.group_id` entfernt.
- Ortsverband-Auswertung: Teilnehmendenzahl durch Stationsanzahl aufgebläht — behoben mit `COUNT(DISTINCT teilnehmer_id)`.
- Teilnehmer-Urkunden: „Gruppenmitglieder"-Label hatte überschüssigen Doppelpunkt und war linksbündig statt zentriert.
- Teilnehmer-Urkunden: Tabellenzeilen drifteten links aus dem Inhaltsbereich — `SetX(contentLeft)` jetzt je Zeile gesetzt.
- Teilnehmer-Urkunden: Linker Rand des Inhaltsbereichs von 5 mm auf 10 mm angepasst.

---

## [0.1.0] — 2026-03-13

### Hinzugefügt

- Erstveröffentlichung von Jugendolympiade Verwaltung.
- Gruppenverwaltung (Gruppen) mit Tab-Ansicht.
- Ergebniseingabe mit gruppen-erstem Arbeitsablauf — Gruppe auswählen, alle Stationen in Tabelle eingeben.
- Einzel- und Massenspeicherung für Stationsergebnisse; bestehende Ergebnisse vorausgefüllt.
- Schnellnavigations-Schaltfläche in Gruppen-Ansicht direkt zur Ergebniseingabe.
- Gruppen-Auswertung (Gruppenwertung) und Ortsverband-Auswertung.
- PDF-Generierung für Gruppen-Auswertung, OV-Auswertung und Urkunden.
- Datenbank-Backup-Funktion.
- Datenbank-Wiederherstellung aus Backup (neueste zuerst sortiert).
- Frontend nach Features gegliedert (shared, admin, groups, stations, evaluations, reports).
