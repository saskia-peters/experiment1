# Gruppenverteilung

Nach erfolgreichem Excel-Import **„Gruppen zusammenstellen"** klicken.

## Verteilungsmodus wählen

Die gewünschte Strategie wird in `config.toml` unter `[verteilung]` eingestellt:

```toml
[verteilung]
# "Klassisch" | "Fahrzeuge" | "FixGroupSize"
verteilungsmodus = "FixGroupSize"
fixgroupsize = 8      # Zielgröße (nur FixGroupSize)
cargroups = "ja"      # Fahrzeugpools (nur FixGroupSize)
```

| Modus | Wann verwenden | Fahrzeuge erforderlich |
|-------|---------------|----------------------|
| **Klassisch** | Keine Fahrzeuge; Gruppen nach `max_groesse` | Nein |
| **Fahrzeuge** | Jede Gruppe bekommt genau ein Fahrzeug | Ja |
| **FixGroupSize** | Feste Zielgröße; Fahrzeuge optional als Pools | Optional |

!!! tip "Empfehlung"
    Der Standard-Modus **FixGroupSize** mit `cargroups = "ja"` ist für die meisten Veranstaltungen die beste Wahl: Gruppen haben gleichmäßige Größe und Fahrzeuge werden gruppenübergreifend optimal gebündelt.

---

## Verteilungsalgorithmen

### Modus: Klassisch

Das Verhalten unterscheidet sich je nachdem, ob Fahrzeuge importiert wurden.

---

#### Ohne Fahrzeuge

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

### Modus: Fahrzeuge (Fahrzeug-zuerst-Pfad)

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

### Modus: FixGroupSize

Verteilt **N** Teilnehmende in Gruppen mit einer festen Zielgröße (`fixgroupsize`).

**Gruppenanzahl** = `round(N / fixgroupsize)`, mindestens 1.  
Innerhalb dieser Anzahl werden die Teilnehmenden gleichmäßig auf ±1 aufgeteilt:

| Beispiel | N | fixgroupsize | Gruppen | Größen |
|----------|---|-------------|---------|--------|
| — | 20 | 8 | 3 | 7, 7, 6 |
| — | 24 | 8 | 3 | 8, 8, 8 |
| — | 25 | 8 | 3 | 9, 8, 8 |
| — | 15 | 6 | 3 (round(2,5)=3) | 5, 5, 5 |

Pre-Groups werden wie im klassischen Pfad zuerst platziert.

#### Fahrzeugzuweisung (cargroups)

Das Verhalten für Fahrzeuge wird durch `cargroups` gesteuert:

=== "cargroups = \"nein\""
    **1:1-Zuweisung** — Gruppen nach Personenzahl absteigend, Fahrzeuge nach Sitzplätzen absteigend. Jede Gruppe bekommt genau ein Fahrzeug (sofern vorhanden).

=== "cargroups = \"ja\" (Standard)"
    **Fahrzeugpools (CarGroups)** — Fahrzeuge werden gruppenübergreifend gebündelt. Mehrere Gruppen teilen sich einen Pool von Fahrzeugen.

    **Algorithmus (DFS + Knapsack-DP, alle Fahrzeuge werden verwendet):**

    1. Fahrzeuge nach Sitzplätzen absteigend sortieren.
    2. **Phase 1 – Optimale Poolbildung (DFS + Backtracking):**
        - Tiefensuche versucht, Gruppen zu kleinen Pools (1–3 Gruppen, 1–5 Fahrzeuge) zusammenzufassen.
        - Für jeden Pool wird via 0/1-Knapsack-DP das Fahrzeug-Subset mit minimalen leeren Sitzen gesucht (Toleranz: 0–3 freie Sitze je Pool).
        - Schlägt die Suche fehl, greift ein Fallback: eine Gruppe pro Pool, Fahrzeuge sequenziell zuweisen.
    3. **Phase 2 – Restfahrzeuge verteilen:** Übrige Fahrzeuge werden dem Pool mit der geringsten freien Kapazität zugeteilt. Kein Fahrzeug bleibt unbenutzt.
    4. **Phase 3 – Fahrer zuweisen:**
        - Fahrzeuge mit bereits eingetragenem Fahrer (aus der XLSX-Datei) werden **unverändert übernommen** — auch wenn dieser Fahrer nicht in der Betreuenden-Liste erscheint (z. B. LKW-Führerschein-Inhaber).
        - Fahrzeuge **ohne** Fahrer erhalten als Fallback die erste verfügbare lizenzierte Betreuende aus dem Pool. Jede Betreuende kann nur einmal als Fahrer eingeteilt werden.

    Das Ergebnis ist ein **CarGroups-PDF** (`CarGroups.pdf`) mit einer Seite je Fahrzeugpool.
    Die Pool-Zuteilung wird in der Datenbank gespeichert und nach einem Backup/Restore automatisch wiederhergestellt.

    | Spalte | Inhalt |
    |--------|--------|
    | Gruppen-Tabelle | Gruppenname, Anzahl TN, Anzahl Betreuende, Gesamt |
    | Fahrzeug-Tabelle | Fahrzeug (OV), Fahrer, Sitzplätze |
    | Fußzeile | Gesamtsitze und freie Plätze je Pool |

---

### Betreuende-Verteilung (vier Phasen)

Gilt für alle drei Verteilungsmodi (Klassisch, Fahrzeuge, FixGroupSize).  
Im Fahrzeug-zuerst-Pfad werden Betreuende, die bereits als Fahrzeugfahrer eingetragen wurden, bei der Verteilung übersprungen — sie sind bereits in ihrer Gruppe.

---

#### Ziel

Das Ziel der Verteilung ist:

1. **Jede Gruppe bekommt mindestens eine Betreuende** und — wenn möglich — mindestens eine Person mit Fahrerlaubnis.
2. **Betreuende aus demselben Ortsverband bleiben möglichst zusammen.** Ein OV-Cluster darf nur aufgeteilt werden, wenn es die Gleichmäßigkeit der Verteilung erfordert.
3. **Gleichmäßige Aufteilung:** Die Anzahl Betreuender pro Gruppe soll um maximal 1 differieren.

---

#### Phase 1 — Lizenzierte Betreuende (Fahrerlaubnis = ja) verteilen

Alle Betreuenden mit Fahrerlaubnis werden zuerst in einer **OV-Round-Robin-Reihenfolge** sortiert: bevor ein OV eine zweite lizenzierte Person platziert, bekommt jeder andere OV erst einmal eine. Dies verhindert, dass ein einzelner OV alle Fahrerpositionen belegt.

Anschließend wird jede lizenzierte Person einzeln zugewiesen:

- **Primär:** die Gruppe mit den wenigsten lizenzierten Betreuenden (Ziel: eine pro Gruppe, bevor jemand eine zweite bekommt).
- **Gleichstand-Tiebreaker:** die Gruppe, in der bereits die meisten Teilnehmenden aus demselben OV sind (die Betreuende soll bei „ihren" TN landen).

> **Beispiel:** OV Berlin schickt 3 lizenzierte Betreuende (A, B, C). Phase 1 verteilt sie in drei verschiedene Gruppen — nicht alle drei in eine.

---

#### Phase 2 — Unlizenzierte Betreuende verteilen

Jede unlizenzierte Betreuende wird der Gruppe zugewiesen, die ihrem OV am nächsten ist. Es wird dabei in dieser Prioritätsreihenfolge gesucht:

| Priorität | Bedingung |
|-----------|-----------|
| 1 | Eine Gruppe, die bereits eine **lizenzierte** Betreuende aus **demselben OV** enthält — bevorzugt die Gruppe mit den wenigsten OV-Mitgliedern (um die Seiten des OV-Clusters auszubalancieren, falls der OV mehrere lizenzierte Personen hat) |
| 2 | Eine Gruppe, die bereits eine **beliebige** Betreuende aus demselben OV enthält |
| 3 | Die Gruppe mit den wenigsten Betreuenden insgesamt |

> **Ziel:** Eine unlizenzierte Betreuende „folgt" ihrer lizenzierten OV-Kollegin in dieselbe Gruppe.

---

#### Phase 2b — Neuausgleich (Rebalancing)

Wenn nach Phase 2 die Gruppe mit den meisten Betreuenden und die mit den wenigsten um mehr als 1 differieren, werden unlizenzierte Personen von der größten in die kleinste Gruppe verschoben — so lange, bis der Unterschied ≤ 1 ist.

**OV-Präferenz beim Verschieben:**

- *Aus der Quellgruppe heraus* wird bevorzugt jemand gewählt, dessen OV in der Quellgruppe noch **≥ 2 unlizenzierte Mitglieder** hat — die Verschiebung zerstört dann den OV-Cluster nicht vollständig.
- *In die Zielgruppe hinein* wird bevorzugt eine Gruppe gewählt, die bereits eine Betreuende aus **demselben OV** hat.

Sind diese Präferenzen nicht erfüllbar, wird trotzdem verschoben (Gleichmäßigkeit geht vor OV-Kohäsion).

---

#### Phase 3 — Sicherheitsnetz

Gruppen, die nach den vorigen Phasen **keine einzige Betreuende** haben, bekommen jemanden aus der Gruppe mit den meisten Betreuenden (Spender muss ≥ 2 haben). Dabei gilt:

- Bevorzugt wird eine **unlizenzierte** Person verschoben, damit die Spendergruppe ihren Fahrer behält.
- Wieder gilt OV-Präferenz: bevorzugt jemand, dessen OV noch ≥ 2 unlizenzierte Mitglieder in der Spendergruppe hat.
- Gibt es nur noch eine einzige Betreuende in allen Gruppen, kann nicht mehr spendet werden — die fehlende Zuweisung wird als Warnung ausgegeben.

---

#### Warum Betreuende trotzdem getrennt werden können

Obwohl der Algorithmus aktiv versucht, OV-Cluster zusammenzuhalten, gibt es Konstellationen, in denen eine Trennung unvermeidbar ist:

| Ursache | Erklärung |
|---------|-----------|
| **Mehrere lizenzierte Personen pro OV** | Phase 1 verteilt sie auf verschiedene Gruppen (Round-Robin). Die unlizenzierten OV-Mitglieder können nur einem von ihnen folgen — der Rest der lizenzierten Fahrer ist schon weg. |
| **Mehr Gruppen als OV-Mitglieder** | Hat ein OV 2 Betreuende, aber es gibt 5 Gruppen, können maximal 2 Gruppen eine OV-Betreuende erhalten. Die übrigen Gruppen bekommen jemanden aus einem anderen OV zugewiesen. |
| **Rebalancing erzwingt Trennung** | Phase 2b verschiebt unlizenzierte Personen aus der größten in die kleinste Gruppe. Wenn die OV-Präferenz (≥ 2 unlizenzierte im Quellcluster) nicht erfüllt ist, wird trotzdem verschoben — der OV-Cluster kann dabei auf 1 Person in einer Gruppe schrumpfen. |
| **Sicherheitsnetz Phase 3** | Muss eine komplett leere Gruppe versorgt werden, wird aus dem nächstbesten Spender genommen, unabhängig vom OV. |
| **Ungünstiges Zahlenverhältnis** | Sind z. B. 3 Betreuende eines OV vorhanden und 4 Gruppen, muss mindestens einer alleine in einer Gruppe sein — egal wie der Algorithmus vorgeht. |

!!! tip "Hinweis für Veranstalter"
    Soll ein OV möglichst vollständig zusammenbleiben, empfiehlt es sich, die Anzahl der Betreuenden eines OV als Vielfaches der Gruppenanzahl zu wählen, oder das `PreGroup`-Feld in der Excel-Datei zu nutzen, um Gruppen explizit vorab zu definieren. PreGroups werden **vor** der Betreuenden-Verteilung festgelegt und bleiben garantiert intakt.

### Warnmeldungen

Nach der Verteilung erscheinen Warnungen für:

- Gruppen **ohne jede Betreuungsperson**.
- Gruppen **ohne Person mit Fahrerlaubnis**.
- Fahrzeuge im Pool **ohne zugewiesenen Fahrer** (Fallback-Zuweisung nicht möglich).
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
- **Fahrzeuge / Fahrzeugpool**: Im normalen Modus die direkt zugewiesenen Fahrzeuge; im CarGroups-Modus die Fahrzeuge des gemeinsamen Pools mit Poolnummer und Hinweis auf gemeinsam reisende Gruppen. Bei fehlender Fahrzeugzuweisung: **„Kein Fahrzeug!"** (roter Hinweis)

!!! info "📸 Screenshot: `groups-view.png`"
    _Gruppenansicht — Tabs mit Teilnehmenden, Betreuenden und Fahrzeugen_

### Eingabeübersicht (Ergebnismatrix)

**„Eingabeübersicht"** zeigt eine Matrix aller Gruppen × Stationen. Klick auf eine Zelle springt direkt zur Ergebniseingabe für diese Kombination.

!!! info "📸 Screenshot: `eingabe-uebersicht.png`"
    _Eingabeübersicht — Gruppen × Stationen Matrix mit ✓/✗ Status_

## Gruppengrößen & Verteilung konfigurieren

In `config.toml` anpassen, danach **„Gruppen zusammenstellen"** erneut klicken:

```toml
[gruppen]
max_groesse = 8   # Maximal-TN pro Gruppe (Klassisch / Fahrzeuge)
min_groesse = 6   # Minimal-TN pro Gruppe (Fahrzeug-Pfad)

[verteilung]
verteilungsmodus = "FixGroupSize"  # "Klassisch" | "Fahrzeuge" | "FixGroupSize"
fixgroupsize = 8                   # Zielgröße (FixGroupSize)
cargroups = "ja"                   # Fahrzeugpools (FixGroupSize)
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
