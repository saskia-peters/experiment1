package main

import (
	"THW-JugendOlympiade/backend/handlers"
)

func (a *App) CheckStartup() map[string]interface{} {
	return handlers.CheckStartup()
}

func (a *App) UseExistingDB() map[string]interface{} {
	return handlers.UseExistingDB(&a.db)
}

func (a *App) ResetToFreshDB() map[string]interface{} {
	return handlers.ResetToFreshDB(&a.db)
}

func (a *App) CheckDB() map[string]interface{} {
	return handlers.CheckDB(a.db)
}

func (a *App) LoadFile() map[string]interface{} {
	return handlers.LoadFile(a.ctx, &a.db)
}

func (a *App) HasScores() bool {
	return handlers.HasScores(a.db)
}

func (a *App) DistributeGroups() map[string]interface{} {
	return handlers.DistributeGroups(a.db, a.cfg.Gruppen.MaxGroesse)
}
