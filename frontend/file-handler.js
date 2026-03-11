// File loading and management
import { setStatus, output, tabs, btnShow, btnStations, btnEvaluation, btnOrtsverband, btnPDF, btnCertificates } from './dom.js';

export async function openFileDialog() {
    try {
        // Check if database has data
        const result = await window.go.main.App.CheckDB();
        
        if (result.hasData) {
            const confirmed = confirm(`The database contains ${result.count} participants. Do you want to discard this data and load a new file?`);
            if (!confirmed) {
                return;
            }
        }
        
        // Open file dialog and load file
        const uploadResult = await window.go.main.App.LoadFile();
        
        if (uploadResult.status === 'error') {
            setStatus('ERROR: ' + uploadResult.message, 'error');
            output.style.display = 'block';
            tabs.style.display = 'none';
            output.textContent = 'Failed to load file. Please check the error message above.';
        } else {
            setStatus(uploadResult.message, 'success');
            btnShow.disabled = false;
            btnStations.disabled = false;
            btnEvaluation.disabled = false;
            btnOrtsverband.disabled = false;
            btnPDF.disabled = false;
            btnCertificates.disabled = false;
            output.style.display = 'block';
            tabs.style.display = 'none';
            output.textContent = `✔ Successfully loaded ${uploadResult.count} participants and created balanced groups!\n\nNext steps:\n• Click "Show Groups" to view the groups\n• Click "Auswertung nach Gruppen" for group evaluation\n• Click "Auswertung nach Ortsverband" for location-based evaluation\n• Click "Generate PDF" to export groups to PDF\n• Click "Teilnehmer-Zertifikate" to generate participant certificates`;
            
            // Collapse Admin and expand other categories
            const adminDropdown = document.querySelector('.button-section:nth-child(1) .category-dropdown');
            const datenDropdown = document.querySelector('.button-section:nth-child(2) .category-dropdown');
            const ausgabeDropdown = document.querySelector('.button-section:nth-child(3) .category-dropdown');
            if (adminDropdown) adminDropdown.removeAttribute('open');
            if (datenDropdown) datenDropdown.setAttribute('open', 'open');
            if (ausgabeDropdown) ausgabeDropdown.setAttribute('open', 'open');
        }
    } catch (err) {
        setStatus('ERROR: ' + err, 'error');
        output.textContent = 'Error: ' + err;
    }
}

export async function handleBackupDatabase() {
    setStatus('Creating database backup...', 'info');
    
    try {
        const result = await window.go.main.App.BackupDatabase();
        
        if (result.status === 'error') {
            setStatus('ERROR: ' + result.message, 'error');
        } else {
            setStatus('✅ ' + result.message, 'success');
        }
    } catch (err) {
        setStatus('ERROR: ' + err, 'error');
    }
}
