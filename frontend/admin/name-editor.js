// name-editor.js — dialog for correcting Teilnehmende and Betreuende names
import { setStatus } from '../shared/dom.js';

export async function handleEditNames() {
    setStatus('Ortsverbände werden geladen...', 'info');
    try {
        const result = await window.go.main.App.GetOrtsverbands();
        if (result.status === 'error') {
            setStatus('FEHLER: ' + result.message, 'error');
            return;
        }
        const ovs = result.ortsverbands || [];
        if (ovs.length === 0) {
            setStatus('Keine Daten vorhanden. Bitte zuerst eine Excel-Datei einlesen.', 'info');
            return;
        }
        setStatus('Bereit.', 'info');
        _openOVPicker(ovs);
    } catch (err) {
        setStatus('FEHLER: ' + err, 'error');
    }
}

// Step 1 – pick an Ortsverband
function _openOVPicker(ovs) {
    const overlay = _makeOverlay('ov-picker-overlay');

    const box = document.createElement('div');
    box.style.cssText = _boxStyle('420px');

    const options = ovs.map(ov =>
        `<option value="${_esc(ov)}">${_esc(ov)}</option>`
    ).join('');

    box.innerHTML = `
        <h2 style="${_h2Style()}">✏️ Namen korrigieren</h2>
        <p style="margin:0 0 16px;color:#555;line-height:1.5">
            Wählen Sie einen Ortsverband, um die Namen der Personen zu bearbeiten.
        </p>
        <select id="_ovSelect" style="width:100%;padding:8px 10px;border:2px solid #ddd;border-radius:6px;font-size:14px;margin-bottom:20px;box-sizing:border-box;">
            ${options}
        </select>
        <div style="display:flex;gap:10px;justify-content:flex-end;">
            <button id="_ovCancel" style="${_btnStyle('#9e9e9e')}">Abbrechen</button>
            <button id="_ovOk"     style="${_btnStyle('#1976d2')}">Weiter</button>
        </div>`;

    overlay.appendChild(box);
    document.body.appendChild(overlay);

    box.querySelector('#_ovCancel').addEventListener('click', () => {
        document.body.removeChild(overlay);
    });
    box.querySelector('#_ovOk').addEventListener('click', async () => {
        const chosen = box.querySelector('#_ovSelect').value;
        document.body.removeChild(overlay);
        await _openPersonEditor(chosen);
    });
}

// Step 2 – show persons for the chosen OV with inline editable name fields
async function _openPersonEditor(ortsverband) {
    setStatus(`Namen für „${ortsverband}" werden geladen...`, 'info');
    try {
        const result = await window.go.main.App.GetPersonenByOrtsverband(ortsverband);
        if (result.status === 'error') {
            setStatus('FEHLER: ' + result.message, 'error');
            return;
        }
        setStatus('Bereit.', 'info');
        const persons = result.persons || [];
        _renderEditor(ortsverband, persons);
    } catch (err) {
        setStatus('FEHLER: ' + err, 'error');
    }
}

function _renderEditor(ortsverband, persons) {
    const overlay = _makeOverlay('name-editor-overlay');

    const box = document.createElement('div');
    box.style.cssText = _boxStyle('640px');

    const kindLabel = { teilnehmende: 'Teilnehmende/r', betreuende: 'Betreuende/r' };

    const rows = persons.map((p, i) => `
        <tr style="border-bottom:1px solid #f0f0f0;">
            <td style="padding:6px 8px;color:#777;font-size:12px;white-space:nowrap;">${_esc(kindLabel[p.kind] || p.kind)}</td>
            <td style="padding:6px 8px;">
                <input
                    data-idx="${i}"
                    data-id="${p.id}"
                    data-kind="${_esc(p.kind)}"
                    data-original="${_esc(p.name)}"
                    type="text"
                    value="${_esc(p.name)}"
                    style="width:100%;padding:5px 7px;border:1px solid #ddd;border-radius:4px;font-size:13px;box-sizing:border-box;"
                />
            </td>
            <td style="padding:6px 8px;width:60px;text-align:center;">
                <span class="_save-status" data-idx="${i}" style="font-size:12px;"></span>
            </td>
        </tr>`).join('');

    box.innerHTML = `
        <h2 style="${_h2Style()}">✏️ Namen korrigieren — ${_esc(ortsverband)}</h2>
        <p style="margin:0 0 12px;color:#666;font-size:13px;">
            Namen bearbeiten und „Speichern" klicken. Änderungen werden sofort in die Datenbank übernommen.
        </p>
        <div style="overflow-y:auto;max-height:55vh;border:1px solid #e0e0e0;border-radius:6px;">
            <table style="width:100%;border-collapse:collapse;">
                <thead>
                    <tr style="background:#f5f5f5;font-size:12px;color:#555;">
                        <th style="padding:8px;text-align:left;font-weight:600;">Rolle</th>
                        <th style="padding:8px;text-align:left;font-weight:600;">Name</th>
                        <th style="padding:8px;"></th>
                    </tr>
                </thead>
                <tbody id="_nameRows">${rows}</tbody>
            </table>
        </div>
        ${persons.length === 0 ? '<p style="color:#888;margin-top:12px;text-align:center;">Keine Personen gefunden.</p>' : ''}
        <div style="display:flex;gap:10px;justify-content:flex-end;margin-top:16px;">
            <button id="_nameBack"  style="${_btnStyle('#9e9e9e')}">← Zurück</button>
            <button id="_nameSave"  style="${_btnStyle('#2e7d32')}"${persons.length === 0 ? ' disabled' : ''}>Alle speichern</button>
            <button id="_nameClose" style="${_btnStyle('#1976d2')}">Schließen</button>
        </div>`;

    overlay.appendChild(box);
    document.body.appendChild(overlay);

    // Highlight changed inputs
    box.querySelectorAll('input[data-id]').forEach(input => {
        input.addEventListener('input', () => {
            input.style.borderColor = input.value !== input.dataset.original ? '#f7971e' : '#ddd';
            const statusEl = box.querySelector(`._save-status[data-idx="${input.dataset.idx}"]`);
            if (statusEl) statusEl.textContent = '';
        });
    });

    box.querySelector('#_nameBack').addEventListener('click', async () => {
        document.body.removeChild(overlay);
        // reload OV list and reopen picker
        const r = await window.go.main.App.GetOrtsverbands();
        if (r.status === 'ok') _openOVPicker(r.ortsverbands || []);
    });

    box.querySelector('#_nameClose').addEventListener('click', () => {
        document.body.removeChild(overlay);
    });

    box.querySelector('#_nameSave').addEventListener('click', async () => {
        const inputs = box.querySelectorAll('input[data-id]');
        let saved = 0;
        let failed = 0;
        for (const input of inputs) {
            const newName = input.value.trim();
            if (newName === input.dataset.original) continue; // unchanged
            const statusEl = box.querySelector(`._save-status[data-idx="${input.dataset.idx}"]`);
            try {
                const r = await window.go.main.App.UpdatePersonName(
                    input.dataset.kind,
                    parseInt(input.dataset.id, 10),
                    newName
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
            } catch (err) {
                if (statusEl) { statusEl.textContent = '✖'; statusEl.style.color = '#c62828'; }
                failed++;
            }
        }
        if (saved === 0 && failed === 0) {
            setStatus('Keine Änderungen.', 'info');
        } else if (failed === 0) {
            setStatus(`✅ ${saved} Name${saved !== 1 ? 'n' : ''} gespeichert.`, 'success');
        } else {
            setStatus(`⚠ ${saved} gespeichert, ${failed} fehlgeschlagen.`, 'error');
        }
    });
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
