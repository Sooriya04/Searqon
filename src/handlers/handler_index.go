package handlers

import (
	"encoding/json"
	"net/http"

	"src/db"
	"src/utils"
)

type IndexSearchRequest struct {
	Query string `json:"query"`
	Limit int    `json:"limit"`
}

// SearchIndexHandler performs full-text query match against previously indexed corpus pages.
func SearchIndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	var req IndexSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeInvalidJSON, "Invalid JSON request body")
		return
	}
	if req.Query == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "Query string is required")
		return
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}

	results, err := db.SearchLocalIndex(req.Query, req.Limit)
	if err != nil {
		utils.WriteError(w, http.StatusInternalServerError, utils.ErrCodeInternalError, "Local index search failed: "+err.Error())
		return
	}

	utils.WriteSuccess(w, results)
}
