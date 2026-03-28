// config-editor.js — modal shell for editing config.toml and certificate layout.
// Graphical editor logic lives in cert-layout-editor.js.
import { setStatus } from '../shared/dom.js';
import {
    gfxPanelHTML,
    loadCertLayoutEditor,
    getCertLayoutData,
    resetCertLayoutEditor,
} from './cert-layout-editor.js';

export async function handleEditConfig() {
    setStatus('Konfiguration wird geladen...', 'info');

    try {
        const [configResult, layoutResult] = await Promise.all([
            window.go.main.App.GetConfigRaw(),
            window.go.main.App.GetCertLayoutRaw(),
        ]);

        if (configResult.status === 'error') {
            setStatus('FEHLER: ' + configResult.message, 'error');
            return;
        }
        if (layoutResult.status === 'error') {
            setStatus('FEHLER: ' + layoutResult.message, 'error');
            return;
        }

        _openModal(configResult.content, layoutResult.content);
        setStatus('Konfiguration geladen.', 'info');
    } catch (err) {
        setStatus('FEHLER: ' + err, 'error');
    }
}

function _openModal(configContent, layoutContent) {
    const overlay = document.createElement('div');
    overlay.id = 'config-editor-overlay';
    overlay.style.cssText = 'position:fixed;top:0;left:0;width:100%;height:100%;background:rgba(0,0,0,0.5);z-index:1000;display:flex;justify-content:center;align-items:center;';

    overlay.innerHTML = `
        <div style="background:white;border-radius:12px;padding:28px 30px;width:min(1000px,96vw);max-height:92vh;display:flex;flex-direction:column;box-shadow:0 20px 60px rgba(0,0,0,0.3);">
            <h2 style="margin:0 0 16px 0;color:#333;">⚙️ Konfiguration bearbeiten</h2>

            <!-- Tab bar -->
            <div style="display:flex;gap:0;border-bottom:2px solid #e0e0e0;margin-bottom:16px;align-items:flex-end;">
                <button id="cfg-tab-config"
                    onclick="window._switchConfigTab('config')"
                    style="padding:8px 20px;border:none;border-bottom:2px solid #667eea;margin-bottom:-2px;background:none;cursor:pointer;font-weight:600;font-size:13px;color:#667eea;">
                    Grundeinstellungen
                </button>
                <button id="cfg-tab-graphical"
                    onclick="window._switchConfigTab('graphical')"
                    style="padding:8px 20px;border:none;border-bottom:2px solid transparent;margin-bottom:-2px;background:none;cursor:pointer;font-weight:400;font-size:13px;color:#888;">
                    Urkundenlayout
                </button>
                <button id="cfg-tab-layout"
                    onclick="window._switchConfigTab('layout')"
                    style="display:none;padding:8px 20px;border:none;border-bottom:2px solid transparent;margin-bottom:-2px;background:none;cursor:pointer;font-weight:400;font-size:13px;color:#888;">
                    Urkundenlayout (technisch)
                </button>
                <button id="cfg-tab-reveal"
                    onclick="window._toggleTechnicalTab()"
                    style="margin-left:auto;padding:4px 10px;border:1px solid #ddd;border-radius:4px;margin-bottom:4px;background:#fafafa;cursor:pointer;font-size:11px;color:#aaa;">
                    ⚙ technisch
                </button>
            </div>

            <!-- Tab: config.toml -->
            <div id="cfg-panel-config" style="display:flex;flex-direction:column;flex:1;overflow:hidden;min-height:0;">
                <p style="margin:0 0 10px 0;color:#666;font-size:13px;">
                    Bearbeiten Sie die <code>config.toml</code>. Zeilen mit <code>#</code> sind Kommentare.
                    Ungültige TOML-Syntax wird vor dem Speichern abgewiesen.
                </p>
                <textarea id="config-editor-textarea"
                    spellcheck="false"
                    style="flex:1;min-height:320px;font-family:'Courier New',monospace;font-size:13px;line-height:1.6;
                           padding:12px;border:2px solid #ddd;border-radius:6px;resize:vertical;color:#333;
                           outline:none;transition:border-color 0.2s;"
                    onfocus="this.style.borderColor='#667eea'"
                    onblur="this.style.borderColor='#ddd'"
                ></textarea>
            </div>

            <!-- Tab: certificate_layout.toml -->
            <div id="cfg-panel-layout" style="display:none;flex-direction:column;flex:1;overflow:hidden;min-height:0;">
                <p style="margin:0 0 10px 0;color:#666;font-size:13px;">
                    Bearbeiten Sie <code>certificate_layout.toml</code>. Diese Datei steuert Position,
                    Schriftgröße und Farbe aller Elemente auf den Urkunden.
                    Ungültiges TOML wird abgewiesen.
                </p>
                <textarea id="layout-editor-textarea"
                    spellcheck="false"
                    style="flex:1;min-height:320px;font-family:'Courier New',monospace;font-size:12px;line-height:1.5;
                           padding:12px;border:2px solid #ddd;border-radius:6px;resize:vertical;color:#333;
                           outline:none;transition:border-color 0.2s;"
                    onfocus="this.style.borderColor='#667eea'"
                    onblur="this.style.borderColor='#ddd'"
                ></textarea>
            </div>

            <!-- Tab: Urkundenlayout (graphical) -->
            <div id="cfg-panel-graphical" style="display:none;flex-direction:column;flex:1;overflow:hidden;min-height:0;">
                ${gfxPanelHTML()}
            </div>

            <div id="config-editor-error" style="display:none;margin-top:10px;padding:10px 14px;background:#ffebee;border-left:4px solid #f44336;border-radius:4px;color:#c62828;font-size:13px;"></div>
            <div style="display:flex;justify-content:flex-end;gap:10px;margin-top:16px;">
                <button onclick="window._closeConfigEditor()"
                    style="padding:10px 22px;background:#e0e0e0;color:#333;border:none;border-radius:6px;cursor:pointer;font-weight:600;font-size:13px;">
                    Abbrechen
                </button>
                <button onclick="window._saveConfig()"
                    style="padding:10px 22px;background:linear-gradient(135deg,#667eea 0%,#764ba2 100%);color:white;border:none;border-radius:6px;cursor:pointer;font-weight:600;font-size:13px;">
                    💾 Speichern
                </button>
            </div>
        </div>
    `;

    document.body.appendChild(overlay);
    document.getElementById('config-editor-textarea').value = configContent;
    document.getElementById('layout-editor-textarea').value = layoutContent;
    // Focus the active (first) tab's textarea
    document.getElementById('config-editor-textarea').focus();
}

window._switchConfigTab = function (tab) {
    ['config', 'layout', 'graphical'].forEach(t => {
        const panel = document.getElementById('cfg-panel-' + t);
        const btn   = document.getElementById('cfg-tab-' + t);
        if (!panel || !btn) return;
        const active = t === tab;
        panel.style.display          = active ? 'flex' : 'none';
        btn.style.borderBottomColor  = active ? '#667eea' : 'transparent';
        btn.style.fontWeight         = active ? '600' : '400';
        btn.style.color              = active ? '#667eea' : '#888';
    });
    // Lazy-load graphical editor on first visit
    if (tab === 'graphical' && !getCertLayoutData()) loadCertLayoutEditor();
    const errorDiv = document.getElementById('config-editor-error');
    if (errorDiv) errorDiv.style.display = 'none';
};

window._closeConfigEditor = function () {
    const overlay = document.getElementById('config-editor-overlay');
    if (overlay) overlay.remove();
    resetCertLayoutEditor();
    setStatus('Bearbeitung abgebrochen.', 'info');
};

window._toggleTechnicalTab = function () {
    const tabBtn  = document.getElementById('cfg-tab-layout');
    const revBtn  = document.getElementById('cfg-tab-reveal');
    if (!tabBtn) return;
    const isVisible = tabBtn.style.display !== 'none';
    if (isVisible) {
        tabBtn.style.display = 'none';
        const layoutPanel = document.getElementById('cfg-panel-layout');
        if (layoutPanel && layoutPanel.style.display !== 'none') {
            window._switchConfigTab('graphical');
        }
        if (revBtn) { revBtn.textContent = '⚙ technisch'; revBtn.style.color = '#aaa'; }
    } else {
        tabBtn.style.display = '';
        window._switchConfigTab('layout');
        if (revBtn) { revBtn.textContent = '✕ technisch'; revBtn.style.color = '#888'; }
    }
};

window._saveConfig = async function () {
    const errorDiv = document.getElementById('config-editor-error');
    errorDiv.style.display = 'none';

    const configPanel    = document.getElementById('cfg-panel-config');
    const graphicalPanel = document.getElementById('cfg-panel-graphical');
    const isConfigTab    = configPanel    && configPanel.style.display    !== 'none';
    const isGraphical    = graphicalPanel && graphicalPanel.style.display !== 'none';

    try {
        let result;
        if (isConfigTab) {
            const ta = document.getElementById('config-editor-textarea');
            if (!ta) return;
            result = await window.go.main.App.SaveConfigRaw(ta.value);
            if (result.status === 'ok') {
                try { window.appConfig = await window.go.main.App.GetConfig(); } catch (_) { /* non-fatal */ }
            }
        } else if (isGraphical) {
            const data = getCertLayoutData();
            if (!data) { errorDiv.textContent = '⚠️ Keine Daten geladen.'; errorDiv.style.display = 'block'; return; }
            result = await window.go.main.App.SaveCertLayoutJSON(JSON.stringify(data));
            if (result.status === 'ok') {
                // Sync raw-TOML textarea so it stays consistent
                try {
                    const raw = await window.go.main.App.GetCertLayoutRaw();
                    if (raw.status === 'ok') {
                        const ta = document.getElementById('layout-editor-textarea');
                        if (ta) ta.value = raw.content;
                    }
                } catch (_) { /* non-fatal */ }
            }
        } else {
            const ta = document.getElementById('layout-editor-textarea');
            if (!ta) return;
            result = await window.go.main.App.SaveCertLayoutRaw(ta.value);
        }

        if (result.status === 'error') {
            errorDiv.textContent = '⚠️ ' + result.message;
            errorDiv.style.display = 'block';
            return;
        }

        const overlay = document.getElementById('config-editor-overlay');
        if (overlay) overlay.remove();
        resetCertLayoutEditor();
        setStatus('✅ ' + result.message, 'success');

    } catch (err) {
        errorDiv.textContent = '⚠️ ' + err;
        errorDiv.style.display = 'block';
    }
};

