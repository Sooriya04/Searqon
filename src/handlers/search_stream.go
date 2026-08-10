package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"src/search"
	"src/utils"
)

// SearchStreamHandler stream search results via SSE as they are scraped.
func SearchStreamHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		utils.WriteError(w, http.StatusInternalServerError, "STREAMING_UNSUPPORTED", "Streaming unsupported")
		return
	}

	query := r.URL.Query().Get("q")
	if strings.TrimSpace(query) == "" {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"Query parameter 'q' is required\"}\n\n")
		flusher.Flush()
		return
	}

	res := search.RunSearchPipeline(query, 5, true, false, 0, true, "")

	discBytes, _ := json.Marshal(map[string]interface{}{
		"query":    query,
		"provider": res.Provider,
		"total":    res.Total,
	})
	fmt.Fprintf(w, "event: discovery\ndata: %s\n\n", discBytes)
	flusher.Flush()

	for _, item := range res.Results {
		itemBytes, _ := json.Marshal(item)
		fmt.Fprintf(w, "event: result\ndata: %s\n\n", itemBytes)
		flusher.Flush()
	}

	doneBytes, _ := json.Marshal(map[string]interface{}{
		"duration_ms": res.Duration,
		"summary":     res.Summary,
	})
	fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneBytes)
	flusher.Flush()
}
