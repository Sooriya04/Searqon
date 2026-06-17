package handlers

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"time"

	"src/scraper"
	"src/utils"
)

type RSSFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel RSSChannel `xml:"channel"`
}

type RSSChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	PubDate     string    `xml:"pubDate,omitempty"`
	Items       []RSSItem `xml:"item"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description,omitempty"`
	Guid        string `xml:"guid,omitempty"`
}

// FeedHandler returns an RSS XML channel generated from a page's links.
func FeedHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		utils.WriteError(w, http.StatusMethodNotAllowed, utils.ErrCodeMethodNotAllowed, "Method not allowed")
		return
	}

	targetURL := r.URL.Query().Get("url")
	if targetURL == "" {
		utils.WriteError(w, http.StatusBadRequest, utils.ErrCodeMissingParam, "url parameter is required")
		return
	}

	result, _ := scraper.ScrapeSingleURL(targetURL, "text", false)
	if result.Error != "" {
		utils.WriteError(w, http.StatusBadGateway, utils.ErrCodeScrapeFailed, result.Error)
		return
	}

	feed := RSSFeed{
		Version: "2.0",
		Channel: RSSChannel{
			Title:       "Searqon Feed: " + result.Title,
			Link:        result.URL,
			Description: "RSS Feed compiled from outbound links on " + result.Domain,
			PubDate:     time.Now().Format(time.RFC1123Z),
		},
	}

	for _, link := range result.OutboundLinks {
		feed.Channel.Items = append(feed.Channel.Items, RSSItem{
			Title:       link,
			Link:        link,
			Guid:        link,
			Description: "Link discovered on " + result.URL,
		})
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, xml.Header)
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(feed)
}
