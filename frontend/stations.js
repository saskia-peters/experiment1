// Station management and display
import { setStatus, output, tabs, tabButtons, tabContents, clearAllTabs } from './dom.js';
import { escapeHtml } from './utils.js';
import { checkForExistingScore } from './scores.js';

export async function handleShowStations() {
    setStatus('Loading stations...', 'info');
    
    try {
        // Get both stations and groups
        const [stationsResult, groupsResult] = await Promise.all([
            window.go.main.App.ShowStations(),
            window.go.main.App.GetAllGroups()
        ]);
        
        if (stationsResult.status === 'error') {
            setStatus('ERROR: ' + stationsResult.message, 'error');
            output.style.display = 'block';
            tabs.style.display = 'none';
            output.textContent = 'Error loading stations: ' + stationsResult.message;
        } else if (groupsResult.status === 'error') {
            setStatus('ERROR: ' + groupsResult.message, 'error');
            output.style.display = 'block';
            tabs.style.display = 'none';
            output.textContent = 'Error loading groups: ' + groupsResult.message;
        } else {
            setStatus('Displaying ' + stationsResult.count + ' stations', 'success');
            output.style.display = 'none';
            tabs.style.display = 'block';
            // Ensure complete cleanup before rendering
            clearAllTabs();
            renderStationTabs(stationsResult.stations, groupsResult.groups);
        }
    } catch (err) {
        setStatus('ERROR: ' + err, 'error');
        output.style.display = 'block';
        tabs.style.display = 'none';
        output.textContent = 'Error: ' + err;
    }
}

function renderStationTabs(stations, groups) {
    // Clear existing tabs - already done by clearAllTabs, but keep for safety
    tabButtons.innerHTML = '';
    tabContents.innerHTML = '';
    
    if (!stations || stations.length === 0) {
        tabContents.innerHTML = '<div style="padding: 20px;">No stations found.</div>';
        return;
    }
    
    // Store stations and groups for later access
    window.currentStations = stations;
    window.currentGroups = groups;
    window.currentStationIndex = 0;
    
    // Create score entry form ABOVE station buttons - full width
    const scoreForm = document.createElement('div');
    scoreForm.id = 'score-entry-form';
    scoreForm.style.cssText = 'background: #f0f8ff; padding: 12px; border-radius: 8px; border: 2px solid #4facfe; margin: 15px -10px 15px -10px; width: calc(100% + 20px); box-sizing: border-box;';
    
    let formHtml = '<h3 id="score-form-title" style="margin: 0 0 15px 0; color: #333; font-size: 16px;">📝 Gruppenergebnis bei Station ' + stations[0].StationName + '</h3>';
    
    // First row: Group selector, score display, and submit button
    formHtml += '<div style="display: flex; gap: 15px; align-items: center; margin-bottom: 15px;">';
    
    // Group selector
    formHtml += '<label style="font-weight: 600; color: #333; font-size: 13px;">Gruppe:</label>';
    formHtml += '<select id="global-group-select" style="padding: 8px; border-radius: 4px; border: 1px solid #ddd; font-size: 13px; min-width: 120px;">';
    formHtml += '<option value="">Auswählen...</option>';
    if (groups && groups.length > 0) {
        groups.forEach(groupID => {
            formHtml += '<option value="' + groupID + '">Gruppe ' + groupID + '</option>';
        });
    }
    formHtml += '</select>';
    
    // Hidden station ID storage
    formHtml += '<input type="hidden" id="global-station-select" data-station-id="' + stations[0].StationID + '" value="' + stations[0].StationName + '">';
    
    // Score display value
    formHtml += '<span id="score-display" style="font-weight: bold; color: #667eea; font-size: 20px; min-width: 70px; text-align: center; background: #f8f9fa; padding: 6px 12px; border-radius: 4px; border: 2px solid #667eea; margin-left: auto;">650</span>';
    
    // Submit button
    formHtml += '<button onclick="window.handleGlobalAssignScore()" style="padding: 8px 16px; background: linear-gradient(135deg, #43e97b 0%, #38f9d7 100%); color: white; border: none; border-radius: 4px; font-weight: 600; cursor: pointer; font-size: 13px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">Speichern</button>';
    
    formHtml += '</div>';
    
    // Second row: Full-width score slider
    formHtml += '<div style="width: 100%;">';
    formHtml += '<label style="font-weight: 600; color: #333; font-size: 13px; display: block; margin-bottom: 8px;">Ergebnis:</label>';
    
    // Container for tick labels and slider - with proper alignment
    formHtml += '<div style="width: 100%; max-width: 100%;">';
    
    // Tick labels above slider - positioned to align with slider stops
    formHtml += '<div style="display: flex; justify-content: space-between; font-size: 10px; color: #666; margin-bottom: 5px; padding: 0 8px;">';
    formHtml += '<span style="width: 30px; text-align: center;">100</span>';
    formHtml += '<span style="width: 30px; text-align: center;">300</span>';
    formHtml += '<span style="width: 30px; text-align: center;">500</span>';
    formHtml += '<span style="width: 30px; text-align: center;">700</span>';
    formHtml += '<span style="width: 30px; text-align: center;">900</span>';
    formHtml += '<span style="width: 30px; text-align: center;">1100</span>';
    formHtml += '<span style="width: 30px; text-align: center;">1200</span>';
    formHtml += '</div>';
    
    // Slider - full width without separate display (display is in first row)
    formHtml += '<input type="range" id="global-score-select" min="100" max="1200" step="50" value="650" list="score-ticks" style="width: 100%; height: 32px; cursor: pointer;" oninput="document.getElementById(\'score-display\').textContent = this.value">';
    formHtml += '<datalist id="score-ticks">';
    for (let i = 100; i <= 1200; i += 50) {
        formHtml += '<option value="' + i + '"></option>';
    }
    formHtml += '</datalist>';
    
    formHtml += '</div>';
    formHtml += '</div>';
    
    // Warning indicator area
    formHtml += '<div id="score-warning" style="margin-top: 10px; padding: 10px; border-radius: 4px; display: none; background: #fff3cd; border: 2px solid #ffc107; color: #856404; font-size: 12px;">';
    formHtml += '<strong>⚠️ Warning:</strong> <span id="score-warning-text"></span>';
    formHtml += '</div>';
    
    scoreForm.innerHTML = formHtml;
    tabButtons.appendChild(scoreForm);
    
    // Add event listener to group selector to check for existing scores
    setTimeout(() => {
        const groupSelect = document.getElementById('global-group-select');
        if (groupSelect) {
            groupSelect.addEventListener('change', checkForExistingScore);
        }
    }, 0);
    
    // Create a 4-column grid of station buttons
    const buttonGrid = document.createElement('div');
    buttonGrid.className = 'station-button-grid';
    buttonGrid.style.cssText = 'display: grid; grid-template-columns: repeat(4, 1fr); gap: 10px; padding: 20px;';
    
    stations.forEach((station, index) => {
        const button = document.createElement('button');
        button.className = 'station-grid-button';
        button.style.cssText = 'padding: 12px; font-size: 14px; font-weight: 600; min-width: 0; background: linear-gradient(135deg, #fbc2eb 0%, #a6c1ee 100%); color: white; border: none; border-radius: 6px; cursor: pointer; transition: all 0.3s; box-shadow: 0 2px 4px rgba(0,0,0,0.1);';
        button.textContent = station.StationName;
        button.onclick = () => showStationDetails(index);
        button.onmouseover = () => { button.style.transform = 'translateY(-1px)'; button.style.boxShadow = '0 4px 8px rgba(0,0,0,0.15)'; };
        button.onmouseout = () => { button.style.transform = 'translateY(0)'; button.style.boxShadow = '0 2px 4px rgba(0,0,0,0.1)'; };
        buttonGrid.appendChild(button);
    });
    
    tabButtons.appendChild(buttonGrid);
    
    // Create content area for station details
    const contentArea = document.createElement('div');
    contentArea.id = 'station-content-area';
    contentArea.style.cssText = 'padding: 20px; border-top: 2px solid #ddd; display: none;';
    tabContents.appendChild(contentArea);
    
    // Show first station by default
    showStationDetails(0);
}

export function showStationDetails(stationIndex) {
    const stations = window.currentStations;
    const groups = window.currentGroups;
    const contentArea = document.getElementById('station-content-area');
    
    if (!stations || !contentArea) return;
    
    window.currentStationIndex = stationIndex;
    const station = stations[stationIndex];
    
    // Update station display in the form
    const stationSelect = document.getElementById('global-station-select');
    if (stationSelect) {
        stationSelect.value = station.StationName;
        stationSelect.setAttribute('data-station-id', station.StationID);
    }
    
    // Update form title with current station name
    const formTitle = document.getElementById('score-form-title');
    if (formTitle) {
        formTitle.textContent = '📝 Gruppenergebnis bei Station ' + station.StationName;
    }
    
    // Check for existing score when station changes
    checkForExistingScore();
    
    contentArea.innerHTML = formatStationContent(station, groups);
    contentArea.style.display = 'block';
    
    // Update button states
    const buttons = document.querySelectorAll('.station-grid-button');
    buttons.forEach((btn, index) => {
        if (index === stationIndex) {
            btn.style.background = 'linear-gradient(135deg, #667eea 0%, #764ba2 100%)';
            btn.style.boxShadow = '0 4px 8px rgba(0,0,0,0.2)';
        } else {
            btn.style.background = 'linear-gradient(135deg, #fbc2eb 0%, #a6c1ee 100%)';
            btn.style.boxShadow = '0 2px 4px rgba(0,0,0,0.1)';
        }
    });
}

function formatStationContent(station, groups) {
    let html = '<h2 style="margin-bottom: 15px; color: #333;">🏆 ' + escapeHtml(station.StationName) + '</h2>';
    
    // Group scores table (score entry form is now at the top)
    html += '<table class="group-table">';
    html += '<thead><tr>';
    html += '<th style="width: 80px;">Rank</th>';
    html += '<th>Group</th>';
    html += '<th style="width: 150px;">Score</th>';
    html += '</tr></thead><tbody>';
    
    if (station.GroupScores && station.GroupScores.length > 0) {
        station.GroupScores.forEach((groupScore, index) => {
            const rankEmoji = index === 0 ? '🥇' : index === 1 ? '🥈' : index === 2 ? '🥉' : (index + 1) + '.';
            html += '<tr>';
            html += '<td style="text-align: center; font-size: 18px;">' + rankEmoji + '</td>';
            html += '<td>Group ' + groupScore.GroupID + '</td>';
            html += '<td style="font-weight: bold; font-size: 18px;">' + groupScore.Score + '</td>';
            html += '</tr>';
        });
    } else {
        html += '<tr><td colspan="3" style="text-align: center; padding: 20px; color: #999;">No scores recorded yet</td></tr>';
    }
    
    html += '</tbody></table>';
    
    // Statistics panel
    if (station.GroupScores && station.GroupScores.length > 0) {
        html += '<div class="stats-panel">';
        html += '<h3>📊 Station Statistics</h3>';
        html += '<div class="stats-grid">';
        
        html += '<div class="stat-item">';
        html += '<strong>Total Groups</strong>';
        html += '<span>' + station.GroupScores.length + '</span>';
        html += '</div>';
        
        // Calculate average score
        const totalScore = station.GroupScores.reduce((sum, gs) => sum + gs.Score, 0);
        const avgScore = (totalScore / station.GroupScores.length).toFixed(1);
        html += '<div class="stat-item">';
        html += '<strong>Average Score</strong>';
        html += '<span>' + avgScore + '</span>';
        html += '</div>';
        
        // Highest score
        html += '<div class="stat-item">';
        html += '<strong>Highest Score</strong>';
        html += '<span>' + station.GroupScores[0].Score + ' (Group ' + station.GroupScores[0].GroupID + ')</span>';
        html += '</div>';
        
        // Lowest score
        const lastScore = station.GroupScores[station.GroupScores.length - 1];
        html += '<div class="stat-item">';
        html += '<strong>Lowest Score</strong>';
        html += '<span>' + lastScore.Score + ' (Group ' + lastScore.GroupID + ')</span>';
        html += '</div>';
        
        html += '</div></div>';
    }
    
    return html;
}
