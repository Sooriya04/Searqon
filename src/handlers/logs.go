package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"src/utils"
)

// LogsHandler serves system log events persisted in SQLite.
func LogsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "Only GET method is supported for /logs")
		return
	}

	q := r.URL.Query()
	limit := 50
	if lStr := q.Get("limit"); lStr != "" {
		if l, err := strconv.Atoi(lStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0
	if oStr := q.Get("offset"); oStr != "" {
		if o, err := strconv.Atoi(oStr); err == nil && o >= 0 {
			offset = o
		}
	}

	level := q.Get("level")
	source := q.Get("source")
	search := q.Get("search")

	logs, total, err := utils.QueryLogs(limit, offset, level, source, search)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, "LOGS_QUERY_FAILED", err.Error())
		return
	}

	if logs == nil {
		logs = []utils.LogRecord{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"total":   total,
		"limit":   limit,
		"offset":  offset,
		"logs":    logs,
	})
}
