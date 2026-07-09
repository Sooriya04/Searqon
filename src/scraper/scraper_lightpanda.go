package scraper

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"

	"src/models"
)

// ScrapeWithLightpanda runs the Lightpanda CLI to execute JS and fetch HTML.
func ScrapeWithLightpanda(targetURL string, userAgent string, binaryPath string, format string, startTime time.Time) (models.ScrapeResult, string, error) {
	startISO := startTime.UTC().Format(time.RFC3339)
	result := models.ScrapeResult{URL: targetURL, StartTime: startISO}

	args := []string{
		"fetch", targetURL,
		"--dump", "html",
		"--wait-until", "load",
	}

	if userAgent != "" {
		args = append(args, "--user-agent", userAgent)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return result, "", fmt.Errorf("lightpanda error: %v, output: %s", err, string(outputBytes))
	}

	htmlOutput := string(outputBytes)
	if strings.TrimSpace(htmlOutput) == "" {
		return result, "", fmt.Errorf("empty HTML output from lightpanda")
	}

	// Feed rendered HTML into the unified content scraping/cleaning engine
	parsedResult := ScrapeHTMLContent(htmlOutput, targetURL, targetURL, format, startTime)
	parsedResult.StatusCode = 200
	parsedResult.ContentType = "text/html"
	parsedResult.ExtractionMethod = "lightpanda"
	parsedResult.Duration = time.Since(startTime).Milliseconds()
	parsedResult.FetchDurationMS = int(parsedResult.Duration)

	log.Printf("[Lightpanda] Successfully scraped and cleaned page (%d words, %dms)", parsedResult.WordCount, parsedResult.Duration)
	return parsedResult, htmlOutput, nil
}
