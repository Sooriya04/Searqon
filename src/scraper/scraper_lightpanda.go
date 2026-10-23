package scraper

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"

	"src/config"
	"src/models"
)

// GetLightpandaCDPURL retrieves the configured Lightpanda CDP WebSocket endpoint if set.
func GetLightpandaCDPURL() string {
	if envURL := os.Getenv("LIGHTPANDA_URL"); envURL != "" {
		return envURL
	}
	if envURL := os.Getenv("LIGHTPANDA_CDP_URL"); envURL != "" {
		return envURL
	}
	cfg := config.Get()
	if cfg != nil && cfg.Scraper.Lightpanda.CDPURL != "" {
		return cfg.Scraper.Lightpanda.CDPURL
	}
	return ""
}

// ScrapeWithLightpanda runs the Lightpanda CLI or connects via CDP to execute JS and fetch HTML.
func ScrapeWithLightpanda(targetURL string, userAgent string, binaryPath string, format string, startTime time.Time, extractSchema string) (models.ScrapeResult, string, error) {
	return ScrapeWithLightpandaProxy(targetURL, userAgent, "", binaryPath, format, startTime, extractSchema)
}

// ScrapeWithLightpandaProxy runs the Lightpanda CLI or remote CDP server with an optional proxy URL.
func ScrapeWithLightpandaProxy(targetURL string, userAgent string, proxyURL string, binaryPath string, format string, startTime time.Time, extractSchema string) (models.ScrapeResult, string, error) {
	// If remote CDP URL is configured and binary is missing or CDP is explicitly targeted, use remote CDP driver
	if binaryPath == "" || strings.HasPrefix(binaryPath, "ws://") || strings.HasPrefix(binaryPath, "http://") {
		cdpURL := binaryPath
		if cdpURL == "" {
			cdpURL = GetLightpandaCDPURL()
		}
		if cdpURL != "" {
			return ScrapeWithLightpandaCDP(cdpURL, targetURL, format, startTime, extractSchema)
		}
	}

	startISO := startTime.UTC().Format(time.RFC3339)
	result := models.ScrapeResult{URL: targetURL, StartTime: startISO}

	args := []string{
		"fetch", targetURL,
		"--dump", "html",
		"--wait-until", "networkalmostidle",
		"--wait-ms", "3500",
		"--terminate-ms", "9000",
	}

	if userAgent != "" && !strings.Contains(userAgent, "Mozilla") {
		args = append(args, "--user-agent", userAgent)
	}

	if proxyURL != "" {
		args = append(args, "--http-proxy", proxyURL)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 11*time.Second)
	defer cancel()

	var stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, binaryPath, args...)
	cmd.Stderr = &stderr
	outputBytes, err := cmd.Output()
	if err != nil {
		return result, "", fmt.Errorf("lightpanda error: %v, stderr: %s", err, stderr.String())
	}

	htmlOutput := string(outputBytes)
	if strings.TrimSpace(htmlOutput) == "" {
		return result, "", fmt.Errorf("empty HTML output from lightpanda")
	}

	// Feed rendered HTML into the unified content scraping/cleaning engine
	parsedResult := ScrapeHTMLContentWithSchema(htmlOutput, targetURL, targetURL, format, startTime, extractSchema)
	parsedResult.StatusCode = 200
	parsedResult.ContentType = "text/html"
	parsedResult.ExtractionMethod = "lightpanda"
	parsedResult.RenderMethod = "lightpanda"
	parsedResult.EscalationTier = string(TierHeadlessBrowser)
	parsedResult.Duration = time.Since(startTime).Milliseconds()
	parsedResult.FetchDurationMS = int(parsedResult.Duration)

	log.Printf("[Lightpanda] Successfully scraped and cleaned page (%d words, %dms)", parsedResult.WordCount, parsedResult.Duration)
	return parsedResult, htmlOutput, nil
}

// ScrapeWithLightpandaCDP connects directly to a Lightpanda headless browser container via WebSocket CDP
func ScrapeWithLightpandaCDP(cdpEndpoint string, targetURL string, format string, startTime time.Time, extractSchema string) (models.ScrapeResult, string, error) {
	startISO := startTime.UTC().Format(time.RFC3339)
	result := models.ScrapeResult{URL: targetURL, StartTime: startISO}

	hostPort := cdpEndpoint
	hostPort = strings.TrimPrefix(hostPort, "ws://")
	hostPort = strings.TrimPrefix(hostPort, "http://")
	hostPort = strings.TrimSuffix(hostPort, "/")
	if !strings.Contains(hostPort, ":") {
		hostPort = hostPort + ":9222"
	}

	dialer := net.Dialer{Timeout: 5 * time.Second}
	conn, err := dialer.Dial("tcp", hostPort)
	if err != nil {
		return result, "", fmt.Errorf("failed to connect to Lightpanda CDP at %s: %w", hostPort, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(12 * time.Second))

	// Send HTTP WebSocket upgrade without Origin header (CDP servers reject Origin headers)
	upgradeReq := fmt.Sprintf("GET / HTTP/1.1\r\n"+
		"Host: %s\r\n"+
		"Upgrade: websocket\r\n"+
		"Connection: Upgrade\r\n"+
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n"+
		"Sec-WebSocket-Version: 13\r\n\r\n", hostPort)

	if _, err := conn.Write([]byte(upgradeReq)); err != nil {
		return result, "", fmt.Errorf("failed to send websocket upgrade to Lightpanda CDP: %w", err)
	}

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return result, "", fmt.Errorf("websocket handshake failed reading header: %w", err)
		}
		if line == "\r\n" {
			break
		}
	}

	sendCDPCmd := func(id int, method string, params map[string]interface{}, sessionID string) error {
		msg := map[string]interface{}{
			"id":     id,
			"method": method,
		}
		if params != nil {
			msg["params"] = params
		}
		if sessionID != "" {
			msg["sessionId"] = sessionID
		}
		b, _ := json.Marshal(msg)
		return writeCDPFrame(conn, b)
	}

	// 1. Create target
	if err := sendCDPCmd(1, "Target.createTarget", map[string]interface{}{"url": "about:blank"}, ""); err != nil {
		return result, "", fmt.Errorf("CDP Target.createTarget failed: %w", err)
	}

	var targetID string
	var sessionID string

	for {
		raw, err := readCDPFrame(reader)
		if err != nil {
			return result, "", fmt.Errorf("error reading target response from CDP: %w", err)
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(raw, &msg); err == nil {
			if id, ok := msg["id"].(float64); ok && int(id) == 1 {
				if res, ok := msg["result"].(map[string]interface{}); ok {
					if tid, ok := res["targetId"].(string); ok {
						targetID = tid
						break
					}
				}
			}
		}
	}

	if targetID == "" {
		return result, "", fmt.Errorf("failed to obtain targetId from Lightpanda CDP")
	}
	defer func() {
		// Clean up target tab on finish
		_ = sendCDPCmd(99, "Target.closeTarget", map[string]interface{}{"targetId": targetID}, "")
	}()

	// 2. Attach to target session
	if err := sendCDPCmd(2, "Target.attachToTarget", map[string]interface{}{
		"targetId": targetID,
		"flatten":  true,
	}, ""); err != nil {
		return result, "", fmt.Errorf("CDP Target.attachToTarget failed: %w", err)
	}

	for {
		raw, err := readCDPFrame(reader)
		if err != nil {
			return result, "", fmt.Errorf("error reading attach response from CDP: %w", err)
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(raw, &msg); err == nil {
			if id, ok := msg["id"].(float64); ok && int(id) == 2 {
				if res, ok := msg["result"].(map[string]interface{}); ok {
					if sid, ok := res["sessionId"].(string); ok {
						sessionID = sid
						break
					}
				}
			}
		}
	}

	if sessionID == "" {
		return result, "", fmt.Errorf("failed to obtain sessionId from Lightpanda CDP")
	}

	// 3. Enable Page events and Navigate
	_ = sendCDPCmd(3, "Page.enable", nil, sessionID)
	if err := sendCDPCmd(4, "Page.navigate", map[string]interface{}{
		"url": targetURL,
	}, sessionID); err != nil {
		return result, "", fmt.Errorf("CDP Page.navigate failed: %w", err)
	}

	// 4. Wait for Page.loadEventFired or timeout
	loadTimeout := time.After(7 * time.Second)
	pageLoaded := false
	for !pageLoaded {
		select {
		case <-loadTimeout:
			pageLoaded = true
		default:
			_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			raw, err := readCDPFrame(reader)
			if err != nil {
				pageLoaded = true
				break
			}
			var msg map[string]interface{}
			if err := json.Unmarshal(raw, &msg); err == nil {
				if m, ok := msg["method"].(string); ok && (m == "Page.loadEventFired" || m == "Page.domContentEventFired") {
					pageLoaded = true
					break
				}
			}
		}
	}

	// Small hydration buffer for JS rendering
	time.Sleep(300 * time.Millisecond)

	// 5. Evaluate document.documentElement.outerHTML
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if err := sendCDPCmd(5, "Runtime.evaluate", map[string]interface{}{
		"expression":    "document.documentElement.outerHTML",
		"returnByValue": true,
	}, sessionID); err != nil {
		return result, "", fmt.Errorf("CDP Runtime.evaluate failed: %w", err)
	}

	var htmlOutput string
	for {
		raw, err := readCDPFrame(reader)
		if err != nil {
			return result, "", fmt.Errorf("error reading HTML evaluation from CDP: %w", err)
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(raw, &msg); err == nil {
			if id, ok := msg["id"].(float64); ok && int(id) == 5 {
				if res, ok := msg["result"].(map[string]interface{}); ok {
					if rObj, ok := res["result"].(map[string]interface{}); ok {
						if val, ok := rObj["value"].(string); ok {
							htmlOutput = val
							break
						}
					}
				}
			}
		}
	}

	if strings.TrimSpace(htmlOutput) == "" {
		return result, "", fmt.Errorf("empty HTML output from Lightpanda CDP")
	}

	// Parse and clean rendered DOM into Markdown
	parsedResult := ScrapeHTMLContentWithSchema(htmlOutput, targetURL, targetURL, format, startTime, extractSchema)
	parsedResult.StatusCode = 200
	parsedResult.ContentType = "text/html"
	parsedResult.ExtractionMethod = "lightpanda_cdp"
	parsedResult.RenderMethod = "lightpanda_cdp"
	parsedResult.EscalationTier = string(TierHeadlessBrowser)
	parsedResult.Duration = time.Since(startTime).Milliseconds()
	parsedResult.FetchDurationMS = int(parsedResult.Duration)

	log.Printf("[Lightpanda CDP] Successfully scraped and cleaned page via CDP (%d words, %dms)", parsedResult.WordCount, parsedResult.Duration)
	return parsedResult, htmlOutput, nil
}

func writeCDPFrame(w io.Writer, payload []byte) error {
	length := len(payload)
	var header []byte
	if length < 126 {
		header = []byte{0x81, byte(0x80 | length)}
	} else if length <= 65535 {
		header = []byte{0x81, byte(0x80 | 126), 0, 0}
		binary.BigEndian.PutUint16(header[2:4], uint16(length))
	} else {
		header = make([]byte, 10)
		header[0] = 0x81
		header[1] = 0x80 | 127
		binary.BigEndian.PutUint64(header[2:10], uint64(length))
	}

	mask := make([]byte, 4)
	if _, err := rand.Read(mask); err != nil {
		return err
	}
	header = append(header, mask...)

	masked := make([]byte, length)
	for i := 0; i < length; i++ {
		masked[i] = payload[i] ^ mask[i%4]
	}

	_, err := w.Write(append(header, masked...))
	return err
}

func readCDPFrame(r io.Reader) ([]byte, error) {
	header := make([]byte, 2)
	if _, err := io.ReadFull(r, header); err != nil {
		return nil, err
	}
	masked := (header[1] & 0x80) != 0
	length := int(header[1] & 0x7F)

	if length == 126 {
		ext := make([]byte, 2)
		if _, err := io.ReadFull(r, ext); err != nil {
			return nil, err
		}
		length = int(binary.BigEndian.Uint16(ext))
	} else if length == 127 {
		ext := make([]byte, 8)
		if _, err := io.ReadFull(r, ext); err != nil {
			return nil, err
		}
		length = int(binary.BigEndian.Uint64(ext))
	}

	var mask []byte
	if masked {
		mask = make([]byte, 4)
		if _, err := io.ReadFull(r, mask); err != nil {
			return nil, err
		}
	}

	payload := make([]byte, length)
	if _, err := io.ReadFull(r, payload); err != nil {
		return nil, err
	}

	if masked {
		for i := 0; i < length; i++ {
			payload[i] ^= mask[i%4]
		}
	}
	return payload, nil
}

