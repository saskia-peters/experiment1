package handlers

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"THW-JugendOlympiade/backend/config"
	"THW-JugendOlympiade/backend/database"
	"THW-JugendOlympiade/backend/io"
	"THW-JugendOlympiade/backend/models"
)

const templateDir = "templates"

// GetConfig returns the user-facing configuration values needed by the frontend.
func GetConfig(cfg config.Config) map[string]interface{} {
	return map[string]interface{}{
		"scoreMin":     cfg.Ergebnisse.MinPunkte,
		"scoreMax":     cfg.Ergebnisse.MaxPunkte,
		"maxGroupSize": cfg.Gruppen.MaxGroesse,
		"eventName":    cfg.Veranstaltung.Name,
		"eventYear":    cfg.Veranstaltung.Jahr,
		"certStyle":    cfg.Ausgabe.UrkunderStil,
	}
}

// GetConfigRaw returns the raw text content of config.toml for in-app editing.
func GetConfigRaw() map[string]interface{} {
	content, err := config.ReadRaw()
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	return map[string]interface{}{"status": "ok", "content": content}
}

// SaveConfigRaw validates content as TOML and writes config.toml.
// It returns the updated Config and the response map.
// The caller is responsible for applying the new config to application state.
func SaveConfigRaw(content string) (config.Config, map[string]interface{}) {
	cfg, err := config.ValidateAndSave(content)
	if err != nil {
		return config.Config{}, map[string]interface{}{"status": "error", "message": err.Error()}
	}
	return cfg, map[string]interface{}{
		"status":  "ok",
		"message": "Konfiguration gespeichert. Einige Änderungen (z. B. Gruppen, Ergebnisse) werden erst nach einem Neustart der App wirksam.",
	}
}

// GetCertLayoutRaw returns the raw TOML content of certificate_layout.toml.
// If the file does not exist the defaults are written and returned.
func GetCertLayoutRaw() map[string]interface{} {
	content, err := io.ReadCertLayoutRaw()
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	return map[string]interface{}{"status": "ok", "content": content}
}

// SaveCertLayoutRaw validates content as TOML and writes certificate_layout.toml.
func SaveCertLayoutRaw(content string) map[string]interface{} {
	if _, err := io.ValidateAndSaveCertLayoutRaw(content); err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": "Zertifikats-Layout gespeichert. Wirksam bei der nächsten PDF-Generierung.",
	}
}

// GetCertLayoutJSON returns the cert layout as structured JSON data (for the graphical editor).
func GetCertLayoutJSON() map[string]interface{} {
	layout, err := io.LoadCertLayout()
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	return map[string]interface{}{"status": "ok", "data": layout}
}

// SaveCertLayoutJSON accepts JSON-encoded layout data from the graphical editor,
// validates it, and saves as certificate_layout.toml.
func SaveCertLayoutJSON(jsonData string) map[string]interface{} {
	var layout io.CertLayoutFile
	if err := json.Unmarshal([]byte(jsonData), &layout); err != nil {
		return map[string]interface{}{"status": "error", "message": "Ung\u00fcltiges JSON: " + err.Error()}
	}
	if err := io.SaveCertLayout(layout); err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	return map[string]interface{}{
		"status":  "ok",
		"message": "Zertifikats-Layout gespeichert. Wirksam bei der n\u00e4chsten PDF-Generierung.",
	}
}

// ListBackgroundImages returns names of PNG/JPG files in templates/.
// The returned names are bare filenames (without directory prefix).
func ListBackgroundImages() map[string]interface{} {
	entries, err := os.ReadDir(templateDir)
	if err != nil {
		// Directory may not exist yet — not an error, just return empty.
		return map[string]interface{}{"status": "ok", "files": []string{}}
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
			files = append(files, e.Name())
		}
	}
	if files == nil {
		files = []string{}
	}
	return map[string]interface{}{"status": "ok", "files": files}
}

// GetImageAsBase64 reads a PNG/JPG and returns it as a data URL.
// It resolves from templates/ first, then falls back to root for
// legacy/static files (e.g. ov_winner_image.png).
// Accepts either a bare filename ("foo.png") or "templates/foo.png".
// Any path traversal attempt is rejected.
func GetImageAsBase64(filename string) map[string]interface{} {
	name := strings.TrimSpace(filename)
	if name == "" {
		return map[string]interface{}{"status": "error", "message": "Ung\u00fcltiger Dateiname"}
	}
	name = strings.TrimPrefix(name, templateDir+"/")
	name = strings.TrimPrefix(name, templateDir+"\\")
	if strings.ContainsAny(name, "/\\") || strings.HasPrefix(name, ".") {
		return map[string]interface{}{"status": "error", "message": "Ung\u00fcltiger Dateiname"}
	}
	lower := strings.ToLower(name)
	if !strings.HasSuffix(lower, ".png") && !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") {
		return map[string]interface{}{"status": "error", "message": "Nur PNG/JPG erlaubt"}
	}
	absDir, err := filepath.Abs(templateDir)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	absFile := filepath.Join(absDir, name)
	if !strings.HasPrefix(absFile, absDir+string(filepath.Separator)) {
		return map[string]interface{}{"status": "error", "message": "Pfad außerhalb des Template-Ordners"}
	}
	data, err := os.ReadFile(absFile)
	if err != nil {
		// Backward compatibility: allow files in cwd for legacy/static assets.
		data, err = os.ReadFile(name)
	}
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	mime := "image/png"
	if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
		mime = "image/jpeg"
	}
	return map[string]interface{}{
		"status":  "ok",
		"dataURL": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data),
	}
}

// ListGroupPictures returns names of image files in the configured picture directory.
// If the directory does not exist yet an empty list is returned (not an error).
func ListGroupPictures(pictureDir string) map[string]interface{} {
	if pictureDir == "" {
		pictureDir = "pictures"
	}
	entries, err := os.ReadDir(pictureDir)
	if err != nil {
		// Directory may not exist yet — not an error, just return empty
		return map[string]interface{}{"status": "ok", "files": []string{}, "dir": pictureDir}
	}
	var files []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		lower := strings.ToLower(e.Name())
		if strings.HasSuffix(lower, ".png") || strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
			files = append(files, e.Name())
		}
	}
	if files == nil {
		files = []string{}
	}
	return map[string]interface{}{"status": "ok", "files": files, "dir": pictureDir}
}

// GetGroupPictureAsBase64 reads a PNG/JPG from the picture directory and returns it as a data URL.
// Rejects filenames with directory separators for security; verifies the resolved path stays
// within the picture directory.
func GetGroupPictureAsBase64(pictureDir, filename string) map[string]interface{} {
	if pictureDir == "" {
		pictureDir = "pictures"
	}
	if strings.ContainsAny(filename, "/\\") || strings.HasPrefix(filename, ".") {
		return map[string]interface{}{"status": "error", "message": "Ungültiger Dateiname"}
	}
	lower := strings.ToLower(filename)
	if !strings.HasSuffix(lower, ".png") && !strings.HasSuffix(lower, ".jpg") && !strings.HasSuffix(lower, ".jpeg") {
		return map[string]interface{}{"status": "error", "message": "Nur PNG/JPG erlaubt"}
	}
	absDir, err := filepath.Abs(pictureDir)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	absFile := filepath.Join(absDir, filename)
	// Guard: resolved path must be strictly inside absDir
	if !strings.HasPrefix(absFile, absDir+string(filepath.Separator)) {
		return map[string]interface{}{"status": "error", "message": "Pfad außerhalb des Bildordners"}
	}
	data, err := os.ReadFile(absFile)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	mime := "image/png"
	if strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg") {
		mime = "image/jpeg"
	}
	return map[string]interface{}{
		"status":  "ok",
		"dataURL": "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(data),
	}
}

// --- Name editor ---

// GetOrtsverbands returns the sorted list of all distinct Ortsverbands.
func GetOrtsverbands(db *sql.DB) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{"status": "error", "message": "Keine Datenbank geöffnet"}
	}
	ovs, err := database.GetDistinctOrtsverbands(db)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	if ovs == nil {
		ovs = []string{}
	}
	return map[string]interface{}{"status": "ok", "ortsverbands": ovs}
}

// GetPersonenByOrtsverband returns all Teilnehmende and Betreuende for the
// given Ortsverband as a JSON-serialisable slice.
func GetPersonenByOrtsverband(db *sql.DB, ortsverband string) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{"status": "error", "message": "Keine Datenbank geöffnet"}
	}
	persons, err := database.GetPersonenByOrtsverband(db, ortsverband)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": err.Error()}
	}
	if persons == nil {
		persons = []database.PersonRecord{}
	}
	return map[string]interface{}{"status": "ok", "persons": persons}
}

// UpdatePersonName updates the name of a single person identified by kind and id.
func UpdatePersonName(db *sql.DB, kind string, id int, newName string) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{"status": "error", "message": "Keine Datenbank geöffnet"}
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return map[string]interface{}{"status": "error", "message": "Name darf nicht leer sein"}
	}
	if err := database.UpdatePersonName(db, kind, id, newName); err != nil {
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Name konnte nicht gespeichert werden: %v", err)}
	}
	return map[string]interface{}{"status": "ok"}
}

// GetAllStations returns all stations (id + name) ordered alphabetically.
func GetAllStations(db *sql.DB) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{"status": "error", "message": "Keine Datenbank geöffnet"}
	}
	stations, err := database.GetStationNamesOrdered(db)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Stationen konnten nicht geladen werden: %v", err)}
	}
	if stations == nil {
		stations = []models.Station{}
	}
	return map[string]interface{}{"status": "ok", "stations": stations}
}

// UpdateStationName updates the name of a single station identified by id.
func UpdateStationName(db *sql.DB, id int, newName string) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{"status": "error", "message": "Keine Datenbank geöffnet"}
	}
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return map[string]interface{}{"status": "error", "message": "Stationsname darf nicht leer sein"}
	}
	if err := database.UpdateStationName(db, id, newName); err != nil {
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Stationsname konnte nicht gespeichert werden: %v", err)}
	}
	return map[string]interface{}{"status": "ok"}
}

// AddStation adds a new station and returns its generated id.
func AddStation(db *sql.DB, name string) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{"status": "error", "message": "Keine Datenbank geöffnet"}
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return map[string]interface{}{"status": "error", "message": "Stationsname darf nicht leer sein"}
	}
	id, err := database.AddStation(db, name)
	if err != nil {
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Station konnte nicht hinzugefügt werden: %v", err)}
	}
	return map[string]interface{}{"status": "ok", "id": id}
}

// DeleteStation removes a station (and its scores) by id.
func DeleteStation(db *sql.DB, id int) map[string]interface{} {
	if db == nil {
		return map[string]interface{}{"status": "error", "message": "Keine Datenbank geöffnet"}
	}
	if err := database.DeleteStation(db, id); err != nil {
		return map[string]interface{}{"status": "error", "message": fmt.Sprintf("Station konnte nicht gelöscht werden: %v", err)}
	}
	return map[string]interface{}{"status": "ok"}
}
