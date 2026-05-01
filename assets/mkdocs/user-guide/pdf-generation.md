# PDF-Ausgabe & Urkunden

Alle erzeugten PDFs werden in `pdf_ordner` gespeichert (Standard: `pdfdocs/`).

!!! info "📸 Screenshot: `pdf-buttons.png`"
    _Ausgabe-Bereich — Übersicht aller PDF-Schaltflächen_

## Verfügbare PDFs

| PDF | Schaltfläche | Beschreibung |
|-----|-------------|--------------|
| Gruppenübersicht | **"Gruppen-PDF erstellen"** | Eine Seite je Gruppe mit Teilnehmenden + Betreuenden |
| Stationslaufzettel | **"Gruppen-PDF erstellen"** | Leere Ergebnisblätter je Station für die manuelle Erfassung |
| OV-Zuteilung | **"Gruppen-PDF erstellen"** | Eine Seite je Ortsverband mit Betreuenden- und Teilnehmendenzuteilung |
| Teilnehmende-Karten | **"Gruppen-PDF erstellen"** | A4-Seiten mit je 4 A6-Karten zum Ausschneiden (Name, OV, Gruppe) |
| Gruppenwertung | **"Gruppenwertung-PDF"** | Gruppen-Rankings mit Gesamtpunktzahl |
| OV-Wertung | **"Ortsverband-Wertung-PDF"** | Ortsverband-Rankings |
| Teilnehmer-Urkunden | **"Urkunden Teilnehmende"** | Eine Urkunde je Teilnehmenden |
| OV-Urkunden | **"Urkunden OV"** | Eine Urkunde je Ortsverband |

!!! info "Hinweis"
    Die Schaltfläche **„Gruppen-PDF erstellen"** erzeugt alle vier oben markierten PDFs gleichzeitig: `Gruppeneinteilung.pdf`, `Stationslaufzettel.pdf`, `OV-Zuteilung.pdf` und `Teilnehmende-Karten.pdf`.

---

## OV-Zuteilung (`OV-Zuteilung.pdf`)

Erzeugt eine Seite pro Ortsverband mit zwei Tabellen:

**Tabelle 1 — Betreuende**

| Spalte | Inhalt |
|--------|--------|
| Betreuende | Name der Person |
| Gruppe | `Gruppe N - Gruppenname` |
| Fahrzeug | Fahrzeugbezeichnung (Funkrufname) der zugeteilten Gruppe |
| Fhr. | `X` wenn diese Person als Fahrerin/Fahrer eingetragen ist |

**Tabelle 2 — Teilnehmende**

| Spalte | Inhalt |
|--------|--------|
| Teilnehmende | Name der Person |
| Gruppe | `Gruppe N - Gruppenname` |

Die Seiten sind nach Ortsverband alphabetisch sortiert. Langer Text wird automatisch kleiner dargestellt, damit er in die Spalte passt.

!!! info "📸 Screenshot: `ov-zuteilung-seite.png`"
    _OV-Zuteilung — Beispielseite mit Betreuenden- und Teilnehmendentabelle_

---

## Teilnehmende-Karten (`Teilnehmende-Karten.pdf`)

A4-Querformat-Seiten mit je vier A6-Karten (2 × 2 Raster). Nach dem Drucken werden die Seiten in vier Teile geschnitten — jede teilnehmende Person erhält eine Karte.

Jede Karte enthält:

- **Name** (groß, fett)
- **Ortsverband** (mittelgroß, grau, direkt unter dem Namen)
- **Gruppe** — `Gruppe N - Gruppenname` (groß, fett)

Die Karten sind nach Ortsverband und dann nach Name sortiert, damit die Stapel für die Ausgabe leicht aufgeteilt werden können. Ein dünner Rahmen markiert die Schnittlinie.

!!! info "📸 Screenshot: `tn-karten-seite.png`"
    _Teilnehmende-Karten — A4-Seite mit vier A6-Karten zum Ausschneiden_

---

## Teilnehmer-Urkunden

Jede teilnehmende Person erhält eine individuelle Urkunde mit:

- Veranstaltungsjahr und Ort
- Name und Ortsverband der Person
- Gruppennummer
- Platzierung (`„1. Platz"`, `„2. Platz"`, … oder `„Teilnahme"` wenn keine Auswertung vorliegt)
- Gruppenmitglieder (Text-Stil) oder Gruppenfoto (Bild-Stil)

### Stile

Gesteuert durch `urkunden_stil` in `config.toml`:

=== "text (Standard)"
    Listet alle Gruppenmitglieder in einer Tabelle unterhalb der Platzierung. Keine externen Dateien benötigt.

=== "picture"
    Bettet ein JPEG-Gruppenfoto statt der Mitgliedertabelle ein.

    Fotos in `bilder_ordner` ablegen (Standard: `pictures/`):

    ```
    pictures/
    ├── group_picture_001.jpg   ← Gruppe 1
    ├── group_picture_002.jpg   ← Gruppe 2
    └── ...
    ```

    Fehlt ein Foto, wird ein grauer Platzhalter angezeigt.

### Optionale Hintergrundvorlage

Datei ablegen unter:

```
templates/background_urkunde_teilnehmende.png
```

Fehlt die Datei, wird ein eingebautes programmatisches Layout verwendet.

**Vorlagenspezifikationen:**

| Eigenschaft | Wert |
|-------------|------|
| Format | PNG |
| Größe | A4 Hochformat — 210 × 297 mm |
| Auflösung | 2480 × 3508 px bei 300 DPI |

**Inhaltszonen (frei lassen für dynamischen Inhalt):**

| Inhalt | Ca. Y-Position |
|--------|---------------|
| Veranstaltungsjahr | ~35 mm von oben |
| Name | ~85 mm von oben |
| Ortsverband | ~105 mm von oben |
| Gruppennummer | ~125 mm von oben |
| Platzierung | ~140 mm von oben |
| Mitgliedertabelle / Foto | ab ~165 mm von oben |

Positionen in `backend/io/pdf_cert_teilnehmende.go` anpassen:

```go
pdf.SetXY(0, 85)  // zweiter Wert = Y-Position in mm
```

---

## OV-Urkunden

Jeder Ortsverband erhält eine Urkunde:

- **Sieger** — der/die OV mit der höchsten Durchschnittspunktzahl erhalten eine **Siegerurkunde** mit `templates/ov_winner_image.png`.
- **Alle anderen** — erhalten eine identische Teilnahmeurkunde (ohne Platzierung).

### Optionale Hintergrundvorlage

```
templates/background_urkunde_ovs.png
```

Gleiches Format wie die Teilnehmer-Urkunde (A4, 300 DPI PNG).

---

## Zertifikat-Layout

Feine Layoutanpassungen sind über `certificate_layout.json` / `certificate_layout.toml` im Projektstamm möglich. Diese Dateien steuern exakte Elementpositionen und sind über den eingebauten **Cert-Layout-Editor** im Admin-Bereich bearbeitbar.
