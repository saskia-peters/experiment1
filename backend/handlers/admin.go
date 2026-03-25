package handlers

import (
	"THW-JugendOlympiade/backend/config"
)

// GetConfig returns the user-facing configuration values needed by the frontend.
func GetConfig(cfg config.Config) map[string]interface{} {
	return map[string]interface{}{
		"scoreMin":     cfg.Ergebnisse.MinPunkte,
		"scoreMax":     cfg.Ergebnisse.MaxPunkte,
		"maxGroupSize": cfg.Gruppen.MaxGroesse,
		"eventName":    cfg.Veranstaltung.Name,
		"eventYear":    cfg.Veranstaltung.Jahr,
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
