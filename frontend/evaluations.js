// Evaluation management and display
import { setStatus, output, tabs, tabButtons, tabContents, clearAllTabs } from './dom.js';
import { escapeHtml } from './utils.js';

export async function handleGroupEvaluation() {
    setStatus('Loading group evaluations...', 'info');
    
    try {
        const result = await window.go.main.App.GetGroupEvaluations();
        
        if (result.status === 'error') {
            setStatus('ERROR: ' + result.message, 'error');
            output.style.display = 'block';
            tabs.style.display = 'none';
            output.textContent = 'Error loading group evaluations: ' + result.message;
        } else {
            setStatus('Displaying evaluations for ' + result.count + ' groups', 'success');
            output.style.display = 'none';
            tabs.style.display = 'block';
            // Ensure complete cleanup before rendering
            clearAllTabs();
            renderGroupEvaluations(result.evaluations);
        }
    } catch (err) {
        setStatus('ERROR: ' + err, 'error');
        output.style.display = 'block';
        tabs.style.display = 'none';
        output.textContent = 'Error: ' + err;
    }
}

export async function handleOrtsverbandEvaluation() {
    setStatus('Loading Ortsverband evaluations...', 'info');
    
    try {
        const result = await window.go.main.App.GetOrtsverbandEvaluations();
        
        if (result.status === 'error') {
            setStatus('ERROR: ' + result.message, 'error');
            output.style.display = 'block';
            tabs.style.display = 'none';
            output.textContent = 'Error loading Ortsverband evaluations: ' + result.message;
        } else {
            setStatus('Displaying evaluations for ' + result.count + ' Ortsverbände', 'success');
            output.style.display = 'none';
            tabs.style.display = 'block';
            // Ensure complete cleanup before rendering
            clearAllTabs();
            renderOrtsverbandEvaluations(result.evaluations);
        }
    } catch (err) {
        setStatus('ERROR: ' + err, 'error');
        output.style.display = 'block';
        tabs.style.display = 'none';
        output.textContent = 'Error: ' + err;
    }
}

function renderGroupEvaluations(evaluations) {
    // Clear existing tabs - already done by clearAllTabs, but keep for safety
    tabButtons.innerHTML = '';
    tabContents.innerHTML = '';
    
    if (!evaluations || evaluations.length === 0) {
        tabContents.innerHTML = '<div style="padding: 20px;">No evaluations found.</div>';
        return;
    }
    
    // Create single content area for all evaluations
    const contentArea = document.createElement('div');
    contentArea.style.cssText = 'padding: 20px;';
    
    let html = '<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">';
    html += '<h2 style="margin: 0; color: #333;">🏆 Group Rankings - Total Scores</h2>';
    html += '<button onclick="handleGenerateGroupEvaluationPDF()" style="padding: 10px 20px; background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%); color: white; border: none; border-radius: 6px; font-weight: 600; cursor: pointer; font-size: 14px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">📄 Generate PDF</button>';
    html += '</div>';
    
    // Evaluation table
    html += '<table class="group-table">';
    html += '<thead><tr>';
    html += '<th style="width: 100px;">Rank</th>';
    html += '<th style="width: 120px;">Group</th>';
    html += '<th>Stations Visited</th>';
    html += '<th>Total Score</th>';
    html += '</tr></thead><tbody>';
    
    evaluations.forEach((evalItem, index) => {
        const rankEmoji = index === 0 ? '🥇' : index === 1 ? '🥈' : index === 2 ? '🥉' : (index + 1) + '.';
        const rowStyle = index < 3 ? 'background: #fff3cd;' : '';
        html += '<tr style="' + rowStyle + '">';
        html += '<td style="text-align: center; font-size: 20px;">' + rankEmoji + '</td>';
        html += '<td style="font-weight: bold; font-size: 16px;">Group ' + evalItem.GroupID + '</td>';
        html += '<td style="text-align: center;">' + evalItem.StationCount + '</td>';
        html += '<td style="font-weight: bold; font-size: 18px; color: #667eea;">' + evalItem.TotalScore + '</td>';
        html += '</tr>';
    });
    
    html += '</tbody></table>';
    
    // Statistics panel
    html += '<div class="stats-panel">';
    html += '<h3>📊 Overall Statistics</h3>';
    html += '<div class="stats-grid">';
    
    html += '<div class="stat-item">';
    html += '<strong>Total Groups</strong>';
    html += '<span>' + evaluations.length + '</span>';
    html += '</div>';
    
    // Calculate overall average
    const totalScore = evaluations.reduce((sum, e) => sum + e.TotalScore, 0);
    const overallAvg = (totalScore / evaluations.length).toFixed(1);
    html += '<div class="stat-item">';
    html += '<strong>Overall Average Score</strong>';
    html += '<span>' + overallAvg + '</span>';
    html += '</div>';
    
    // Highest score
    html += '<div class="stat-item">';
    html += '<strong>Highest Score</strong>';
    html += '<span>' + evaluations[0].TotalScore + ' (Group ' + evaluations[0].GroupID + ')</span>';
    html += '</div>';
    
    // Lowest score
    const lastEval = evaluations[evaluations.length - 1];
    html += '<div class="stat-item">';
    html += '<strong>Lowest Score</strong>';
    html += '<span>' + lastEval.TotalScore + ' (Group ' + lastEval.GroupID + ')</span>';
    html += '</div>';
    
    html += '</div></div>';
    
    contentArea.innerHTML = html;
    tabContents.appendChild(contentArea);
}

function renderOrtsverbandEvaluations(evaluations) {
    // Clear existing tabs - already done by clearAllTabs, but keep for safety
    tabButtons.innerHTML = '';
    tabContents.innerHTML = '';
    
    if (!evaluations || evaluations.length === 0) {
        tabContents.innerHTML = '<div style="padding: 20px;">No evaluations found.</div>';
        return;
    }
    
    // Create single content area for all evaluations
    const contentArea = document.createElement('div');
    contentArea.style.cssText = 'padding: 20px;';
    
    let html = '<div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 20px;">';
    html += '<h2 style="margin: 0; color: #333;">🏆 Ortsverband Rankings - Average Scores</h2>';
    html += '<button onclick="handleGenerateOrtsverbandEvaluationPDF()" style="padding: 10px 20px; background: linear-gradient(135deg, #4facfe 0%, #00f2fe 100%); color: white; border: none; border-radius: 6px; font-weight: 600; cursor: pointer; font-size: 14px; box-shadow: 0 2px 4px rgba(0,0,0,0.1);">📄 Generate PDF</button>';
    html += '</div>';
    
    // Evaluation table
    html += '<table class="group-table">';
    html += '<thead><tr>';
    html += '<th style="width: 100px;">Rank</th>';
    html += '<th>Ortsverband</th>';
    html += '<th>Participants</th>';
    html += '<th>Total Score</th>';
    html += '<th>Average Score</th>';
    html += '</tr></thead><tbody>';
    
    evaluations.forEach((evalItem, index) => {
        const rankEmoji = index === 0 ? '🥇' : index === 1 ? '🥈' : index === 2 ? '🥉' : (index + 1) + '.';
        const rowStyle = index < 3 ? 'background: #fff3cd;' : '';
        html += '<tr style="' + rowStyle + '">';
        html += '<td style="text-align: center; font-size: 20px;">' + rankEmoji + '</td>';
        html += '<td style="font-weight: bold; font-size: 16px;">' + escapeHtml(evalItem.Ortsverband) + '</td>';
        html += '<td style="text-align: center;">' + evalItem.ParticipantCount + '</td>';
        html += '<td style="text-align: center;">' + evalItem.TotalScore + '</td>';
        html += '<td style="font-weight: bold; font-size: 18px; color: #667eea;">' + evalItem.AverageScore.toFixed(1) + '</td>';
        html += '</tr>';
    });
    
    html += '</tbody></table>';
    
    // Statistics panel
    html += '<div class="stats-panel">';
    html += '<h3>📊 Overall Statistics</h3>';
    html += '<div class="stats-grid">';
    
    html += '<div class="stat-item">';
    html += '<strong>Total Ortsverbände</strong>';
    html += '<span>' + evaluations.length + '</span>';
    html += '</div>';
    
    // Highest average score
    html += '<div class="stat-item">';
    html += '<strong>Highest Average Score</strong>';
    html += '<span>' + evaluations[0].AverageScore.toFixed(1) + ' (' + escapeHtml(evaluations[0].Ortsverband) + ')</span>';
    html += '</div>';
    
    // Lowest average score
    const lastEval = evaluations[evaluations.length - 1];
    html += '<div class="stat-item">';
    html += '<strong>Lowest Average Score</strong>';
    html += '<span>' + lastEval.AverageScore.toFixed(1) + ' (' + escapeHtml(lastEval.Ortsverband) + ')</span>';
    html += '</div>';
    
    // Overall average score across all ortsverbände
    const totalScore = evaluations.reduce((sum, e) => sum + e.TotalScore, 0);
    const totalParticipants = evaluations.reduce((sum, e) => sum + e.ParticipantCount, 0);
    const overallAvg = totalParticipants > 0 ? (totalScore / totalParticipants).toFixed(1) : '0.0';
    html += '<div class="stat-item">';
    html += '<strong>Overall Average Score</strong>';
    html += '<span>' + overallAvg + '</span>';
    html += '</div>';
    
    html += '</div></div>';
    
    contentArea.innerHTML = html;
    tabContents.appendChild(contentArea);
}
