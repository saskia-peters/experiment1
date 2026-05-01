// station-editor.js — dialog for adding, renaming, and removing stations
import { setStatus } from '../shared/dom.js';

export async function handleEditStations() {
    setStatus('Stationen werden geladen...', 'info');
    try {
        const [stationsResult, hasScores] = await Promise.all([
            window.go.main.App.GetAllStations(),
            window.go.main.App.HasScores(),
        ]);
        if (stationsResult.status === 'error') {
            setStatus('FEHLER: ' + stationsResult.message, 'error');
            return;
        }
        const stations = stationsResult.stations || [];
        if (stations.length === 0) {
            setStatus('Keine Stationen vorhanden. Bitte zuerst eine Excel-Datei einlesen.', 'info');
            return;
        }
        setStatus('Bereit.', 'info');
        _renderEditor(stations, hasScores);
    } catch (err) {
        setStatus('FEHLER: ' + err, 'error');
    }
}

function _renderEditor(stations, locked) {
    const overlay = _makeOverlay('station-editor-overlay');

    const box = document.createElement('div');
    box.style.cssText = _boxStyle('620px');

    const lockedNote = locked
        ? `<div style="background:#fff3cd;border:1px solid #ffc107;border-radius:6px;padding:8px 12px;margin-bottom:12px;font-size:12px;color:#856404;">
               ⚠ Hinzufügen, Umbenennen und Löschen ist gesperrt, weil bereits Ergebnisse eingetragen wurden.
           </div>`
        : '';

    box.innerHTML = `
        <h2 style="${_h2Style()}">✏️ Stationen bearbeiten</h2>
        ${lockedNote}
        <p style="margin:0 0 12px;color:#666;font-size:13px;">
            ${locked
                ? 'Stationsliste (nur Ansicht).'
                : 'Namen bearbeiten, Stationen hinzufügen oder entfernen. „Alle speichern" übernimmt Umbenennungen.'}
        </p>
        <div style="overflow-y:auto;max-height:52vh;border:1px solid #e0e0e0;border-radius:6px;">
            <table style="width:100%;border-collapse:collapse;" id="_stationTable">
                <thead>
                    <tr style="background:#f5f5f5;font-size:12px;color:#555;">
                        <th style="padding:8px;text-align:left;font-weight:600;">Stationsname</th>
                        <th style="padding:8px;width:50px;"></th>
                        <th style="padding:8px;width:36px;"></th>
                    </tr>
                </thead>
                <tbody id="_stationRows"></tbody>
            </table>
        </div>
        ${!locked ? `
        <div style="margin-top:12px;display:flex;gap:8px;align-items:center;">
            <input id="_newStationInput" type="text" placeholder="Neuer Stationsname…"
                style="flex:1;padding:7px 10px;border:1px solid #ddd;border-radius:6px;font-size:13px;box-sizing:border-box;" />
            <button id="_addStation" style="${_btnStyle('#1565c0')}">+ Hinzufügen</button>
        </div>` : ''}
        <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:16px;">
            ${!locked ? `<button id="_stationSave" style="${_btnStyle('#2e7d32')}">Alle speichern</button>` : ''}
            <button id="_stationClose" style="${_btnStyle('#757575')}">Schließen</button>
        </div>`;

    overlay.appendChild(box);
    document.body.appendChild(overlay);

    // Render rows into tbody
    const tbody = box.querySelector('#_stationRows');
    stations.forEach(s => _appendRow(tbody, s.StationID, s.StationName, locked));

    // Close button
    box.querySelector('#_stationClose').addEventListener('click', () => {
        document.body.removeChild(overlay);
    });

    if (!locked) {
        // Save all renames
        box.querySelector('#_stationSave').addEventListener('click', async () => {
            const inputs = tbody.querySelectorAll('input[data-id]');
            let saved = 0, failed = 0;
            for (const input of inputs) {
                const newName = input.value.trim();
                if (newName === input.dataset.original) continue;
                const statusEl = input.closest('tr').querySelector('._save-status');
                try {
                    const r = await window.go.main.App.UpdateStationName(
                        parseInt(input.dataset.id, 10), newName
                    );
                    if (r.status === 'ok') {
                        input.dataset.original = newName;
                        input.style.borderColor = '#ddd';
                        if (statusEl) { statusEl.textContent = '✔'; statusEl.style.color = '#2e7d32'; }
                        saved++;
                    } else {
                        if (statusEl) { statusEl.textContent = '✖'; statusEl.style.color = '#c62828'; }
                        failed++;
                    }
                } catch {
                    if (statusEl) { statusEl.textContent = '✖'; statusEl.style.color = '#c62828'; }
                    failed++;
                }
            }
            if (saved === 0 && failed === 0) setStatus('Keine Änderungen.', 'info');
            else if (failed === 0) setStatus(`✅ ${saved} Stationsname${saved !== 1 ? 'n' : ''} gespeichert.`, 'success');
            else setStatus(`⚠ ${saved} gespeichert, ${failed} fehlgeschlagen.`, 'error');
        });

        // Add new station
        const newInput = box.querySelector('#_newStationInput');
        const doAdd = async () => {
            const name = newInput.value.trim();
            if (!name) return;
            try {
                const r = await window.go.main.App.AddStation(name);
                if (r.status === 'ok') {
                    _appendRow(tbody, r.id, name, false);
                    newInput.value = '';
                    newInput.style.borderColor = '#ddd';
                    setStatus(`✅ Station „${name}" hinzugefügt.`, 'success');
                } else {
                    setStatus('FEHLER: ' + r.message, 'error');
                }
            } catch (err) {
                setStatus('FEHLER: ' + err, 'error');
            }
        };
        box.querySelector('#_addStation').addEventListener('click', doAdd);
        newInput.addEventListener('keydown', e => { if (e.key === 'Enter') doAdd(); });
    }
}

// Appends a single station row to tbody
function _appendRow(tbody, id, name, locked) {
    const tr = document.createElement('tr');
    tr.dataset.stationId = id;
    tr.style.borderBottom = '1px solid #f0f0f0';

    if (locked) {
        tr.innerHTML = `
            <td style="padding:6px 8px;font-size:13px;">${_esc(name)}</td>
            <td></td><td></td>`;
    } else {
        tr.innerHTML = `
            <td style="padding:6px 8px;">
                <input
                    data-id="${id}"
                    data-original="${_esc(name)}"
                    type="text"
                    value="${_esc(name)}"
                    style="width:100%;padding:5px 7px;border:1px solid #ddd;border-radius:4px;font-size:13px;box-sizing:border-box;"
                />
            </td>
            <td style="padding:6px 4px;text-align:center;width:50px;">
                <span class="_save-status" style="font-size:12px;"></span>
            </td>
            <td style="padding:6px 4px;text-align:center;width:36px;">
                <button class="_del-btn" title="Station löschen"
                    style="background:none;border:none;cursor:pointer;color:#c62828;font-size:16px;padding:2px 6px;border-radius:4px;">✕</button>
            </td>`;

        const input = tr.querySelector('input');
        input.addEventListener('input', () => {
            input.style.borderColor = input.value !== input.dataset.original ? '#f7971e' : '#ddd';
            const statusEl = tr.querySelector('._save-status');
            if (statusEl) statusEl.textContent = '';
        });

        tr.querySelector('._del-btn').addEventListener('click', async () => {
            const stationName = input.value.trim() || name;
            if (!confirm(`Station „${stationName}" wirklich löschen?`)) return;
            try {
                const r = await window.go.main.App.DeleteStation(parseInt(id, 10));
                if (r.status === 'ok') {
                    tr.remove();
                    setStatus(`Station „${stationName}" gelöscht.`, 'success');
                } else {
                    setStatus('FEHLER: ' + r.message, 'error');
                }
            } catch (err) {
                setStatus('FEHLER: ' + err, 'error');
            }
        });
    }

    tbody.appendChild(tr);
}

// ---- helpers ----

function _makeOverlay(id) {
    const el = document.createElement('div');
    el.id = id;
    el.style.cssText = 'position:fixed;inset:0;background:rgba(0,0,0,.55);display:flex;align-items:center;justify-content:center;z-index:9999';
    return el;
}

function _boxStyle(maxW) {
    return `background:#fff;border-radius:10px;padding:28px 32px;max-width:${maxW};width:94vw;box-shadow:0 8px 32px rgba(0,0,0,.3);font-family:Arial,sans-serif;`;
}

function _h2Style() {
    return 'margin:0 0 12px;font-size:1.15em;color:#333;';
}

function _btnStyle(bg) {
    return `padding:9px 20px;background:${bg};color:#fff;border:none;border-radius:6px;cursor:pointer;font-weight:600;font-size:13px;`;
}

function _esc(str) {
    return String(str)
        .replace(/&/g, '&amp;')
        .replace(/"/g, '&quot;')
        .replace(/</g, '&lt;')
        .replace(/>/g, '&gt;');
}
