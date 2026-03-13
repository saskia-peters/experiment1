// Main application orchestrator - imports and wires up all modules
import { openFileDialog, handleBackupDatabase, handleRestoreDatabase } from './file-handler.js';
import { handleShowGroups } from './groups.js';
import { handleShowStations, handleShowStationsForGroup } from './stations.js';
import { handleGroupEvaluation, handleOrtsverbandEvaluation } from './evaluations.js';
import { 
    handleGeneratePDF, 
    handleGenerateGroupEvaluationPDF, 
    handleGenerateOrtsverbandEvaluationPDF, 
    handleGenerateCertificates 
} from './pdf-handlers.js';

// Expose functions to window object for onclick handlers
window.openFileDialog = openFileDialog;
window.handleBackupDatabase = handleBackupDatabase;
window.handleRestoreDatabase = handleRestoreDatabase;
window.handleShowGroups = handleShowGroups;
window.handleShowStations = handleShowStations;
window.handleShowStationsForGroup = handleShowStationsForGroup;
window.handleEvaluation = handleGroupEvaluation;
window.handleOrtsverbandEvaluation = handleOrtsverbandEvaluation;
window.handleGeneratePDF = handleGeneratePDF;
window.handleGenerateGroupEvaluationPDF = handleGenerateGroupEvaluationPDF;
window.handleGenerateOrtsverbandEvaluationPDF = handleGenerateOrtsverbandEvaluationPDF;
window.handleGenerateCertificates = handleGenerateCertificates;
