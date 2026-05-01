# Excel-Import

Die Anwendung liest alle Veranstaltungsdaten aus einer einzigen XLSX-Datei. Die Datei muss **zwei Pflicht-Tabellenblätter** (`Teilnehmende` und `Stationen`) enthalten. Die Blätter `Betreuende` und `Fahrzeuge` sind optional — fehlen sie, wird der Import trotzdem akzeptiert.

## Tabellenblatt 1: `Teilnehmende`

Alle angemeldeten Teilnehmenden. Die **erste Zeile ist die Kopfzeile**.

| Spalte | Pflichtfeld | Beschreibung |
|--------|-------------|--------------|
| `Name` | ✅ Ja | Vor- und Nachname |
| `Ortsverband` | ✅ Ja | Lokale Gliederung |
| `Alter` | ✅ Ja | Alter als ganze Zahl (0–150) |
| `Geschlecht` | ✅ Ja | `M`, `W` oder `D` (auch ausgeschrieben) — **muss manuell in der XLSX-Datei eingetragen werden**, da dieser Wert beim Export aus dem Anmeldesystem nicht automatisch übernommen wird |
| `PreGroup` | ⬜ Optional | Gruppierschlüssel — Personen mit gleichem Code kommen in dieselbe Gruppe |

!!! info "📸 Screenshot: `xlsx-sheet-teilnehmende.png`"
    _Tabellenblatt „Teilnehmende" — Kopfzeile und erste Datenzeilen in Excel/LibreOffice_

**PreGroup-Regeln:**

- Nur Buchstaben und Ziffern — keine Sonderzeichen oder Leerzeichen.
- Maximale Länge: 20 Zeichen.
- Eine PreGroup, die `max_groesse` überschreiten würde, wird beim Import abgelehnt.

**Beispiel:**

| Name | Ortsverband | Alter | Geschlecht | PreGroup |
|------|-------------|-------|------------|----------|
| Max Mustermann | Berlin-Mitte | 14 | M | |
| Lena Schmidt | Hamburg-Nord | 13 | W | Team1 |
| Jonas Weber | Hamburg-Nord | 15 | M | Team1 |
| Sara Yilmaz | München-Süd | 14 | W | |

---

## Tabellenblatt 2: `Betreuende`

Alle Betreuungspersonen. Die **erste Zeile ist die Kopfzeile**.

| Spalte | Pflichtfeld | Beschreibung |
|--------|-------------|--------------|
| `Name` | ✅ Ja | Name der Betreuungsperson |
| `Ortsverband` | ✅ Ja | Lokale Gliederung |
| `Fahrerlaubnis` | ✅ Ja | `ja` oder `nein` (Groß-/Kleinschreibung irrelevant) |

!!! info "📸 Screenshot: `xlsx-sheet-betreuende.png`"
    _Tabellenblatt „Betreuende" — Kopfzeile und erste Datenzeilen in Excel/LibreOffice_

!!! note "Fahrerlaubnis"
    Der Verteilungsalgorithmus garantiert **mindestens eine Person mit Fahrerlaubnis pro Gruppe**. Sind nicht genug solcher Personen vorhanden, erscheint nach der Verteilung eine Warnmeldung.

**Beispiel:**

| Name | Ortsverband | Fahrerlaubnis |
|------|-------------|---------------|
| Anna Meier | Berlin-Mitte | ja |
| Klaus Bauer | Hamburg-Nord | nein |
| Maria Koch | Hamburg-Nord | ja |

---

## Tabellenblatt 3: `Stationen` (Optional)

Stationsnamen für die Ergebniseingabe. Zeile 1 = Kopfzeile, ab Zeile 2 ein Stationsname pro Zeile.

!!! info "📸 Screenshot: `xlsx-sheet-stationen.png`"
    _Tabellenblatt „Stationen" — Kopfzeile und Stationsnamen in Excel/LibreOffice_

!!! note "Standard-Stationen"
    Fehlt das Tabellenblatt `Stationen` oder ist es leer, wird der Import **nicht** abgelehnt. Stattdessen werden automatisch folgende acht Standard-Stationen geladen und eine Warnmeldung angezeigt:

    | Standard-Station |
    |------------------|
    | Kübelspritze Zielschiessen |
    | Magnetlabyrinth |
    | Merkfähigkeit |
    | Geräuschlabyrinth |
    | Rittersport |
    | Koordiniertes Nageln |
    | Wurfknoten werfen |
    | Polarexpedition |

    Die Stationen können anschließend über **Admin → „Stationen umbenennen"** angepasst werden.

---

## Tabellenblatt 4: `Fahrzeuge` (Optional)

Fahrzeuge für den Transport der Gruppen. Die **erste Zeile ist die Kopfzeile**. Das Blatt kann weggelassen werden — Gruppen erhalten dann keinen Fahrzeug-Eintrag.

| Spalte | Pflichtfeld | Beschreibung |
|--------|-------------|--------------|
| `Bezeichnung` | ✅ Ja | Name/Beschreibung des Fahrzeugs |
| `Ortsverband` | ✅ Ja | Lokale Gliederung |
| `Funkrufname` | ✅ Ja | Funkrufname des Fahrzeugs |
| `Fahrer` | ⬜ Optional | Name des Fahrers — muss in der Betreuenden-Liste mit Fahrerlaubnis vorhanden sein. Bleibt das Feld leer, wird beim Verteilen automatisch die erste lizenzierte Betreuende der Gruppe als Fahrerin eingetragen. |
| `Sitzplaetze` | ✅ Ja | Gesamtzahl Sitzplätze inkl. Fahrer (positive ganze Zahl) |

!!! info "📸 Screenshot: `xlsx-sheet-fahrzeuge.png`"
    _Tabellenblatt „Fahrzeuge" — Kopfzeile und erste Datenzeilen in Excel/LibreOffice_

!!! note "Mindestgröße"
    Fahrzeuge, deren Sitzplatzzahl minus 1 (Fahrerplatz) kleiner als `min_groesse` ist, werden beim Verteilen ausgeschlossen und in einer Warnung gemeldet.

**Beispiel:**

| Bezeichnung | Ortsverband | Funkrufname | Fahrer | Sitzplaetze |
|-------------|-------------|-------------|--------|-------------|
| THW-Transporter | Berlin-Mitte | BER-TH 1 | Klaus Bauer | 9 |
| Mannschaftsbus | Hamburg-Nord | HH-TH 3 | Maria Koch | 16 |

---

## Import durchführen

1. **📝 Daten → "Excel einlesen"** klicken.
2. XLSX-Datei im Dateidialog auswählen.
3. Grüne Statusmeldung = Erfolg. Rote Meldung = Validierungsfehler mit Zeilenangabe.

!!! info "📸 Screenshot: `excel-import-success.png`"
    _Grüne Statusmeldung nach erfolgreichem Import_

---

## Master-Excel umwandeln (Admin)

Liegt die Teilnehmerliste in einem internen Quellformat vor (z. B. direkt aus dem Anmeldesystem), kann der Admin-Bereich sie automatisch in das oben beschriebene Import-Format umwandeln.

### Vorgehen

1. **Admin → „Master-Excel umwandeln"** klicken.
2. Im Dialog **Veranstaltungstyp** wählen: **Jugend** oder **Mini**.
3. Quell-XLSX-Datei auswählen.
4. Speicherort für die erzeugte Ziel-XLSX angeben.
5. Die erzeugte Datei kann direkt mit **„Excel einlesen"** importiert werden.

### Erwartetes Quellformat

**Tabellenblatt `Teilnehmende`** (Quell-Excel):

| Spalte | Beschreibung |
|--------|--------------|
| `Vorname` | Vorname |
| `Name` | Nachname — wird mit `Vorname` zu einem Vollnamen zusammengeführt |
| `Betreuende` | `x` = Betreuungsperson |
| `JuHe` | `x` = Jugend-Teilnehmende (nur für Jugend-Veranstaltung ausgewertet) |
| `Mini` | `x` = Mini-Teilnehmende oder Mini-Betreuende |
| `Alter` | Alter als ganze Zahl |
| `Ortsverband` | Lokale Gliederung |
| `Fahrerlaubnis` | Jeder nicht-leere Wert außer `"/"` gilt als gültige Fahrerlaubnis |
| `Geschlecht` | `m`, `w` oder `d` — wird direkt in die Ziel-XLSX übernommen |

**Tabellenblatt `Fahrzeuge`** (nur Jugend-Veranstaltung):

| Spalte | Beschreibung |
|--------|--------------|
| `Fahrzeug` | Fahrzeugbezeichnung |
| `Ortsverband` | Lokale Gliederung |
| `Funkrufname` | Funkrufname |
| `Fahrer` | Name des Fahrers |
| `Anzahl Plätze incl. Fahrende` | Sitzplatzanzahl als ganze Zahl |

### Zuordnungsregeln

=== "Jugend"
    | Quellzeile | Zielblatt |
    |------------|-----------|
    | `JuHe = x` | `Teilnehmende` |
    | `Betreuende = x` UND `Mini` leer | `Betreuende` |
    | Alle Fahrzeug-Zeilen | `Fahrzeuge` |

=== "Mini"
    | Quellzeile | Zielblatt |
    |------------|-----------|
    | `Mini = x` UND `Betreuende` leer | `Teilnehmende` |
    | `Betreuende = x` UND `Mini = x` | `Betreuende` |
    | *(kein Fahrzeugblatt)* | `Fahrzeuge` enthält nur Kopfzeile |

!!! warning "Bestehende Daten werden überschrieben"
    Ein erneuter Import ersetzt **alle** Daten (inkl. Gruppen und Ergebnisse). Vorher Datenbank sichern.

## Validierungsregeln

| Regel | Fehlermeldung |
|-------|--------------|
| Name darf nicht leer sein | `row N: name is required` |
| Alter muss eine Zahl sein | `row N: age must be a number` |
| Alter muss 0–150 sein | `row N: age must be between 0 and 150` |
| Fahrerlaubnis muss `ja`/`nein` sein | `row N: fahrerlaubnis must be 'ja' or 'nein'` |
| PreGroup enthält ungültige Zeichen | `row N: pregroup contains invalid characters` |
| PreGroup zu lang | `row N: pregroup exceeds 20 characters` |

## Beispieldatei

Eine Beispieldatei mit allen drei Blättern in der korrekten Struktur wird beim ersten Start automatisch nach `example/example_data.xlsx` extrahiert.
