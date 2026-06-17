package scraper

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os/exec"
	"strings"
	"time"

	"src/models"
	"src/utils"
)

// ScrapeWithLightpanda runs the Lightpanda CLI to execute JS and fetch markdown.
func ScrapeWithLightpanda(targetURL string, userAgent string, binaryPath string, format string, startTime time.Time) (models.ScrapeResult, string, error) {
	startISO := startTime.UTC().Format(time.RFC3339)
	result := models.ScrapeResult{URL: targetURL, StartTime: startISO}

	args := []string{
		"fetch", targetURL,
		"--dump", "markdown",
		"--wait-until", "networkalmostidle",
		"--strip-mode", "js,ui,css",
	}

	if userAgent != "" {
		args = append(args, "--user-agent", userAgent)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binaryPath, args...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		return result, "", fmt.Errorf("lightpanda error: %v, output: %s", err, string(outputBytes))
	}

	markdownOutput := string(outputBytes)
	if strings.TrimSpace(markdownOutput) == "" {
		return result, "", fmt.Errorf("empty output from lightpanda")
	}

	title := "Scraped Page"
	lines := strings.Split(markdownOutput, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			title = strings.TrimPrefix(trimmed, "# ")
			break
		}
	}

	plainText := markdownToPlainText(markdownOutput)

	result.Title = title
	result.Content = plainText
	result.Markdown = markdownOutput
	result.WordCount = utils.CountWords(plainText)
	result.EndTime = time.Now().UTC().Format(time.RFC3339)
	result.Duration = time.Since(startTime).Milliseconds()

	if parsedBase, pErr := url.Parse(targetURL); pErr == nil {
		result.Domain = parsedBase.Hostname()
	}
	result.CanonicalURL = targetURL
	result.StatusCode = 200
	result.ContentType = "text/html"
	result.Scraped = true
	result.ExtractionMethod = "lightpanda"
	result.FetchDurationMS = int(result.Duration)

	log.Printf("[Lightpanda] Successfully scraped page (%d words, %dms)", result.WordCount, result.Duration)
	return result, markdownOutput, nil
}

func markdownToPlainText(md string) string {
	lines := strings.Split(md, "\n")
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		line = strings.TrimLeft(line, "#")
		line = strings.TrimSpace(line)
		line = strings.ReplaceAll(line, "**", "")
		line = strings.ReplaceAll(line, "*", "")
		line = strings.ReplaceAll(line, "`", "")
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, " ")
}
