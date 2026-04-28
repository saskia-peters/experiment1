# Gruppenverteilung

Nach erfolgreichem Excel-Import **„Gruppen zusammenstellen"** klicken.

## Verteilungsalgorithmus

Das Verhalten unterscheidet sich je nachdem, ob Fahrzeuge importiert wurden.

---

### Ohne Fahrzeuge (klassischer Pfad)

1. **Pre-Groups werden zuerst platziert** — Teilnehmende mit demselben `PreGroup`-Code kommen in eine dedizierte Gruppe.
2. **Gruppenanzahl** = `anzahlPreGroups + ceil(übrigenTeilnehmende / max_groesse)`.
3. **Restliche Teilnehmende** werden per Diversity-Score verteilt:

    | Faktor | Gewicht |
    |--------|---------|
    | Gleicher Ortsverband bereits in Gruppe | ×2,0 Malus |
    | Gleiches Geschlecht bereits in Gruppe | ×1,5 Malus |
    | Alter nah am Gruppenaltersdurchschnitt | +1,0 Malus |
    | Gruppengröße | +0,5 je Mitglied |

    Jede Person kommt in die Gruppe mit dem **niedrigsten Malus**.

4. **Betreuende** werden anschließend gleichmäßig verteilt (siehe [Betreuende-Verteilung](#betreuende-verteilung-vier-phasen) weiter unten).

---

### Mit Fahrzeugen (Fahrzeug-zuerst-Pfad)

Wenn Fahrzeuge importiert wurden, startet ein mehrstufiger Algorithmus, der sicherstellt, dass jede Gruppe **genau in ihr Fahrzeug passt**.

```mermaid
flowchart TD
    A([Start]) --> B{Fahrzeuge\nvorhanden?}

    B -- Nein --> NV1[Gruppen nach max_groesse bilden\nbalanciert nach OV / Geschlecht / Alter]
    NV1 --> NV2[Betreuende gleichmäßig verteilen]
    NV2 --> SAVE([Speichern & fertig])

    B -- Ja --> P0

    subgraph P0 ["Phase 0 – Fahrzeuge vorbereiten"]
        direction TB
        V0A[Fahrzeuge sortieren\nOV → Bezeichnung]
        V0B{seats − 1 < min_groesse?}
        V0A --> V0B
        V0B -- Ja --> V0C[Fahrzeug ausschließen\nWarnung ausgeben]
        V0B -- Nein --> V0D[Eligible-Liste]
        V0D --> V0E{Zu viele Fahrzeuge für\n⌊TN / min_groesse⌋?}
        V0E -- Ja --> V0F[Überschuss kappen\nWarnung ausgeben]
        V0E -- Nein --> V0G[numGroups = len eligible]
        V0F --> V0G
    end

    P0 --> P1

    subgraph P1 ["Phase 1 – Fahrzeuge & Fahrer zuweisen"]
        direction TB
        V1A[Fahrzeug i → Gruppe i\n1:1-Zuweisung]
        V1B{"Fahrer in Betreuende-Liste?\n(Name + OV übereinstimmend,\nFahrerlaubnis = ja)"}
        V1A --> V1B
        V1B -- Ja --> V1C[Fahrer als Betreuende\nin Gruppe eintragen]
        V1B -- Nein --> V1D[Warnung: Fahrer nicht gefunden]
    end

    P1 --> P2

    subgraph P2 ["Phase 2 – Teilnehmende verteilen"]
        direction TB
        V2A[PreGroups best-fit\ngleicher OV bevorzugt]
        V2B[Restliche TN nach\nOV / Geschlecht / Alter sortiert]
        V2C[Best-fit mit effectiveCapacity\n= min·max_groesse·seats−Betreuende]
        V2D{Überlauf?}
        V2E[+1-Ausnahme nutzen\nfalls Fahrzeugsitze reichen]
        V2F[Least-full-Fallback\nKapazitätswarnung]
        V2A --> V2B --> V2C --> V2D
        V2D -- Nein --> P3
        V2D -- Ja --> V2E
        V2E --> P3
        V2D -- Kein Platz --> V2F --> P3
    end

    subgraph P3 ["Phase 3 – Betreuende & Ausgleich"]
        direction TB
        V3A[Verbleibende Betreuende\ngleichmäßig verteilen\nPhase 3]
        V3B{Gruppe ohne\nzugewiesenen Fahrer?}
        V3C[Erste lizenzierte Betreuende\nder Gruppe als Fahrer setzen\nPhase 3b]
        V3D[Überlastete Gruppen entlasten\nTN oder Betreuende verschieben\nPhase 3c]
        V3E[Betreuende:TN-Verhältnis\nausgleichen via Tausch\nPhase 3d]
        V3A --> V3B
        V3B -- Ja --> V3C --> V3D
        V3B -- Nein --> V3D
        V3D --> V3E
    end

    P2 --> P3
    P3 --> P4

    subgraph P4 ["Phase 4 – Kapazitätsprüfung & Warnungen"]
        V4A[Gesamtsitzplätze vs. Gesamtpersonen]
        V4B[Pro-Gruppe: Personen vs. Sitzplätze]
        V4A --> V4B
    end

    P4 --> SAVE
```

#### Schlüsselbegriffe

| Begriff | Bedeutung |
|---------|-----------|
| `max_groesse` | Harte Obergrenze für Teilnehmende pro Gruppe |
| `min_groesse` | Untergrenze: Fahrzeuge mit weniger Passagierplätzen werden ausgeschlossen (Standard: 6) |
| `effectiveCapacity` | `min(max_groesse, Sitzplätze − Betreuende-Anzahl)` — tatsächlich verfügbare TN-Plätze |
| +1-Ausnahme | Hat ein Fahrzeug mehr Sitze als `max_groesse`, werden die Extra-Plätze für Überlauf-TN genutzt |
| Phase 3c | Verschiebt TN (oder Betreuende) von übervollen in Gruppen mit Reserveplätzen |
| Phase 3d | Tauscht eine Betreuende ↔ TN zwischen der Gruppe mit dem höchsten und dem niedrigsten Betreuenden:TN-Verhältnis; Gesamtgröße je Gruppe bleibt konstant |

---

### Betreuende-Verteilung (vier Phasen)

Gilt für beide Pfade (klassisch und Fahrzeug-zuerst). Im Fahrzeug-Pfad werden bereits als Fahrer eingetragene Betreuende übersprungen.

1. **Phase 1** — Personen mit Fahrerlaubnis gleichmäßig verteilen: eine Person pro Gruppe in Prioritätsreihenfolge.
2. **Phase 2** — Personen ohne Fahrerlaubnis folgen ihrem Ortsverband: bevorzugt die Gruppe mit einer lizenzierten Person aus demselben OV.
3. **Phase 2b** — Neuausgleich: Personen ohne FL von der größten in die kleinste Gruppe verschieben, bis der Unterschied ≤ 1 ist.
4. **Phase 3 (Sicherheitsnetz)** — Gruppen ohne Betreuende erhalten eine Person aus der größten Gruppe.

### Warnmeldungen

Nach der Verteilung erscheinen Warnungen für:

- Gruppen **ohne jede Betreuungsperson**.
- Gruppen **ohne Person mit Fahrerlaubnis**.
- Fahrzeuge, deren **Fahrer nicht in der Betreuenden-Liste** gefunden wurde.
- Fahrzeuge, die wegen `min_groesse` **ausgeschlossen** wurden.
- Fahrzeuge, die wegen **zu wenig Teilnehmenden** nicht genutzt werden konnten.
- Überschreitung der **Gesamtsitzplatzkapazität**.

Die Verteilung wird trotzdem gespeichert. Durch Anpassen der Excel-Datei und erneuten Import können die Warnungen behoben werden.

---

## Gruppen anzeigen

### Gruppen-Tab

**„Gruppen anzeigen"** öffnet die Tabellen-Ansicht. Jeder Tab zeigt:

- Teilnehmende mit Alter, Geschlecht, Ortsverband
- Betreuende mit Fahrerlaubnis-Status (Fahrer eines Fahrzeugs erscheinen hier ebenfalls)
- Fahrzeuge mit Fahrer, Sitzplätzen und Kapazitätsanzeige; bei fehlender Fahrzeugzuweisung: **„Kein Fahrzeug!"** (roter Hinweis)
- Gruppenstatistik (Anzahl, Geschlechterverteilung, OV-Verteilung)

!!! info "📸 Screenshot: `groups-view.png`"
    _Gruppenansicht — Tabs mit Teilnehmenden, Betreuenden und Fahrzeugen_

### Eingabeübersicht (Ergebnismatrix)

**„Eingabeübersicht"** zeigt eine Matrix aller Gruppen × Stationen. Klick auf eine Zelle springt direkt zur Ergebniseingabe für diese Kombination.

!!! info "📸 Screenshot: `eingabe-uebersicht.png`"
    _Eingabeübersicht — Gruppen × Stationen Matrix mit ✓/✗ Status_

## Gruppengrößen konfigurieren

In `config.toml` anpassen, danach **„Gruppen zusammenstellen"** erneut klicken:

```toml
[gruppen]
max_groesse = 8   # Maximal-TN pro Gruppe
min_groesse = 6   # Minimal-TN pro Gruppe (Fahrzeug-Pfad)
```

## Gruppennamen anpassen

Über `gruppen.gruppennamen` in `config.toml` können benutzerdefinierte Gruppenbezeichnungen vergeben werden (z. B. Gerätegattungen wie „Hebekissen", „Steckleiter", …). Die Namen erscheinen in der Gruppen-Ansicht, der Ergebniseingabe, der Auswertung und auf Teilnehmer-Urkunden.

```toml
[gruppen]
gruppennamen = ["Hebekissen", "Rüstholz", "Tauchpumpe"]
```

Sind weniger Namen als Gruppen vorhanden, wird für fehlende Einträge automatisch **„Gruppe N"** verwendet.

!!! warning
    Neuverteilung ist nur möglich, **bevor das erste Ergebnis gespeichert wurde**. Danach ist die Schaltfläche gesperrt.

## Gruppen-PDF erzeugen

**📊 Ausgabe → „Gruppen-PDF erstellen"** erzeugt ein mehrseitiges PDF (eine Seite je Gruppe) mit allen Teilnehmenden, Betreuenden und dem zugewiesenen Fahrzeug. Datei wird in `pdf_ordner` gespeichert.
