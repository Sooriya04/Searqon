package handlers

import (
	"net/http"

	"src/utils"
)

// StatsHandler exposes running performance telemetry metrics.
func StatsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}
	utils.WriteSuccess(w, utils.GetStats())
}
