// Main application orchestrator - imports and wires up all modules
import { openFileDialog } from './file-handler.js';
import { handleShowGroups } from './groups.js';
import { handleShowStations, showStationDetails } from './stations.js';
import { handleGroupEvaluation, handleOrtsverbandEvaluation } from './evaluations.js';
import { 
    handleGeneratePDF, 
    handleGenerateGroupEvaluationPDF, 
    handleGenerateOrtsverbandEvaluationPDF, 
    handleGenerateCertificates 
} from './pdf-handlers.js';
import { handleGlobalAssignScore, handleAssignScore } from './scores.js';

// Expose functions to window object for onclick handlers
window.openFileDialog = openFileDialog;
window.handleShowGroups = handleShowGroups;
window.handleShowStations = handleShowStations;
window.handleEvaluation = handleGroupEvaluation;
window.handleOrtsverbandEvaluation = handleOrtsverbandEvaluation;
window.handleGeneratePDF = handleGeneratePDF;
window.handleGenerateGroupEvaluationPDF = handleGenerateGroupEvaluationPDF;
window.handleGenerateOrtsverbandEvaluationPDF = handleGenerateOrtsverbandEvaluationPDF;
window.handleGenerateCertificates = handleGenerateCertificates;

// Expose for station and score functionality
window.showStationDetails = showStationDetails;
window.handleGlobalAssignScore = handleGlobalAssignScore;
window.handleAssignScore = handleAssignScore;
