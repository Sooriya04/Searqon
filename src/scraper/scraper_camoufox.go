package scraper

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"time"

	"src/models"
)

// LoadCamoufoxConfig checks environment variables and system PATH for Camoufox installation.
func LoadCamoufoxConfig() (bool, string) {
	if envVal := os.Getenv("CAMOUFOX_ENABLED"); envVal == "true" || envVal == "1" {
		bin := os.Getenv("CAMOUFOX_PATH")
		if bin == "" {
			bin = os.Getenv("CAMOUFOX_BINARY_PATH")
		}
		if bin != "" {
			return true, bin
		}
		if p, err := exec.LookPath("camoufox"); err == nil {
			return true, p
		}
		return true, "camoufox"
	}

	if p, err := exec.LookPath("camoufox"); err == nil {
		return true, p
	}

	return false, ""
}

// ScrapeWithCamoufox executes Camoufox (stealth Firefox anti-detect browser) to render pages and bypass anti-bot gates.
func ScrapeWithCamoufox(targetURL string, proxyURL string, format string, startTime time.Time, extractSchema string) (models.ScrapeResult, string, error) {
	startISO := startTime.UTC().Format(time.RFC3339)
	result := models.ScrapeResult{URL: targetURL, StartTime: startISO}

	enabled, binPath := LoadCamoufoxConfig()
	if !enabled || binPath == "" {
		return result, "", fmt.Errorf("camoufox is not installed or enabled")
	}

	args := []string{"fetch", targetURL, "--headless"}
	if proxyURL != "" {
		args = append(args, "--proxy", proxyURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, args...)
	outputBytes, err := cmd.CombinedOutput()
	if err != nil {
		// Try python module fallback if standalone CLI failed
		if pyBin, pyErr := exec.LookPath("python3"); pyErr == nil {
			pyScript := fmt.Sprintf(`
import sys
try:
    from camoufox.sync_api import Camoufox
    proxy_dict = None
    if %q:
        proxy_dict = {"server": %q}
    with Camoufox(headless=True, proxy=proxy_dict) as browser:
        page = browser.new_page()
        page.goto(%q, wait_until="domcontentloaded", timeout=7000)
        sys.stdout.write(page.content())
except Exception as e:
    sys.stderr.write(str(e))
    sys.exit(1)
`, proxyURL, proxyURL, targetURL)
			pyCmd := exec.CommandContext(ctx, pyBin, "-c", pyScript)
			pyOut, pyRunErr := pyCmd.CombinedOutput()
			if pyRunErr == nil && len(strings.TrimSpace(string(pyOut))) > 0 {
				outputBytes = pyOut
				err = nil
			} else {
				return result, "", fmt.Errorf("camoufox execution failed: %v, output: %s", err, string(outputBytes))
			}
		} else {
			return result, "", fmt.Errorf("camoufox execution failed: %v, output: %s", err, string(outputBytes))
		}
	}

	htmlOutput := string(outputBytes)
	if strings.TrimSpace(htmlOutput) == "" {
		return result, "", fmt.Errorf("empty HTML output from camoufox")
	}

	parsedResult := ScrapeHTMLContentWithSchema(htmlOutput, targetURL, targetURL, format, startTime, extractSchema)
	parsedResult.StatusCode = 200
	parsedResult.ContentType = "text/html"
	parsedResult.RenderMethod = "camoufox"
	parsedResult.EscalationTier = string(TierHeadlessBrowser)
	parsedResult.Duration = time.Since(startTime).Milliseconds()
	parsedResult.FetchDurationMS = int(parsedResult.Duration)

	log.Printf("[Camoufox] Successfully rendered page (%d words, %dms)", parsedResult.WordCount, parsedResult.Duration)
	return parsedResult, htmlOutput, nil
}
