package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

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

	var streamMu sync.Mutex
	safeWrite := func(event string, data []byte) {
		streamMu.Lock()
		defer streamMu.Unlock()
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = strings.TrimSpace(r.URL.Query().Get("q"))
	}
	if query == "" {
		safeWrite("error", []byte(`{"error":"Query parameter 'query' or 'q' is required"}`))
		return
	}

	search.RunSearchPipelineStream(r.Context(), query, 5, true, false, 0, true, "", func(evt search.SearchEvent) {
		switch evt.Type {
		case "discovery":
			discBytes, _ := json.Marshal(map[string]interface{}{
				"query":    query,
				"provider": evt.Provider,
				"total":    evt.Total,
			})
			safeWrite("discovery", discBytes)

		case "scrape":
			if evt.Result != nil {
				itemBytes, _ := json.Marshal(evt.Result)
				safeWrite("scrape", itemBytes)
			}

		case "result":
			if evt.Result != nil {
				itemBytes, _ := json.Marshal(evt.Result)
				safeWrite("result", itemBytes)
			}

		case "synthesis":
			dataBytes, _ := json.Marshal(evt.Data)
			safeWrite("synthesis", dataBytes)

		case "summary":
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"summary": evt.Summary,
			})
			safeWrite("summary", dataBytes)

		case "done":
			doneBytes, _ := json.Marshal(evt.Data)
			safeWrite("done", doneBytes)
		}
	})
}
