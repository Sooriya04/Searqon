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

	query := strings.TrimSpace(r.URL.Query().Get("query"))
	if query == "" {
		query = strings.TrimSpace(r.URL.Query().Get("q"))
	}
	if query == "" {
		fmt.Fprintf(w, "event: error\ndata: {\"error\":\"Query parameter 'query' or 'q' is required\"}\n\n")
		flusher.Flush()
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
			fmt.Fprintf(w, "event: discovery\ndata: %s\n\n", discBytes)
			flusher.Flush()

		case "scrape":
			if evt.Result != nil {
				itemBytes, _ := json.Marshal(evt.Result)
				fmt.Fprintf(w, "event: scrape\ndata: %s\n\n", itemBytes)
				flusher.Flush()
			}

		case "result":
			if evt.Result != nil {
				itemBytes, _ := json.Marshal(evt.Result)
				fmt.Fprintf(w, "event: result\ndata: %s\n\n", itemBytes)
				flusher.Flush()
			}

		case "synthesis":
			dataBytes, _ := json.Marshal(evt.Data)
			fmt.Fprintf(w, "event: synthesis\ndata: %s\n\n", dataBytes)
			flusher.Flush()

		case "summary":
			dataBytes, _ := json.Marshal(map[string]interface{}{
				"summary": evt.Summary,
			})
			fmt.Fprintf(w, "event: summary\ndata: %s\n\n", dataBytes)
			flusher.Flush()

		case "done":
			doneBytes, _ := json.Marshal(evt.Data)
			fmt.Fprintf(w, "event: done\ndata: %s\n\n", doneBytes)
			flusher.Flush()
		}
	})
}
