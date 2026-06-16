package main

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// ─── Robots.txt Structures and Cache ─────────────────────────────────────────

type AgentRules struct {
	DisallowedPaths []string
	AllowedPaths    []string
	CrawlDelay      time.Duration
}

type RobotsData struct {
	Rules map[string]*AgentRules // key: lowercase user-agent name
}

var robotsCache = make(map[string]*RobotsData)
var robotsMu sync.RWMutex

var agentUserAgents = map[string]string{
	"googlebot":   "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)",
	"bingbot":     "Mozilla/5.0 (compatible; bingbot/2.0; +http://www.bing.com/bingbot.htm)",
	"slurp":       "Mozilla/5.0 (compatible; Yahoo! Slurp; http://help.yahoo.com/help/us/ysearch/slurp)",
	"duckduckbot": "Mozilla/5.0 (compatible; DuckDuckBot-Https/1.1; +http://duckduckgo.com/duckduckbot.html)",
	"baiduspider": "Mozilla/5.0 (compatible; Baiduspider/2.0; +http://www.baidu.com/search/spider.html)",
	"yandexbot":   "Mozilla/5.0 (compatible; YandexBot/3.0; +http://yandex.com/bots)",
}

var rotatingUserAgents = []string{
	// Chrome Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
	// Chrome macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Safari/537.36",
	// Firefox Windows
	"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:123.0) Gecko/20100101 Firefox/123.0",
	// Firefox macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10.15; rv:123.0) Gecko/20100101 Firefox/123.0",
	// Safari macOS
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Safari/605.1.15",
	// Safari iOS
	"Mozilla/5.0 (iPhone; CPU iPhone OS 17_3 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/17.3 Mobile/15E148 Safari/605.1.15",
	// Chrome Android
	"Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/122.0.0.0 Mobile Safari/537.36",
}

func getRobotsData(baseURL *url.URL) *RobotsData {
	host := baseURL.Host
	robotsMu.RLock()
	data, exists := robotsCache[host]
	robotsMu.RUnlock()
	if exists {
		return data
	}

	robotsURL := fmt.Sprintf("%s://%s/robots.txt", baseURL.Scheme, host)
	data = &RobotsData{
		Rules: make(map[string]*AgentRules),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", robotsURL, nil)
	if err == nil {
		req.Header.Set("User-Agent", "SearqonBot/1.0 (+https://sooriya04.github.io/Searqon/)")
		resp, rerr := httpClient.Do(req)
		if rerr == nil {
			defer resp.Body.Close()
			if resp.StatusCode == 200 {
				body, berr := io.ReadAll(resp.Body)
				if berr == nil {
					lines := strings.Split(string(body), "\n")
					
					var activeAgents []string
					lastWasAgent := false

					for _, line := range lines {
						line = strings.TrimSpace(line)
						if line == "" || strings.HasPrefix(line, "#") {
							continue
						}
						parts := strings.SplitN(line, ":", 2)
						if len(parts) < 2 {
							continue
						}
						key := strings.ToLower(strings.TrimSpace(parts[0]))
						val := strings.TrimSpace(parts[1])

						if key == "user-agent" {
							if !lastWasAgent {
								activeAgents = []string{}
							}
							agent := strings.ToLower(val)
							activeAgents = append(activeAgents, agent)
							if _, exists := data.Rules[agent]; !exists {
								data.Rules[agent] = &AgentRules{}
							}
							lastWasAgent = true
						} else {
							lastWasAgent = false
							for _, agent := range activeAgents {
								rules := data.Rules[agent]
								if key == "disallow" {
									if val != "" {
										rules.DisallowedPaths = append(rules.DisallowedPaths, val)
									}
								} else if key == "allow" {
									if val != "" {
										rules.AllowedPaths = append(rules.AllowedPaths, val)
									}
								} else if key == "crawl-delay" {
									var delaySec float64
									if _, derr := fmt.Sscanf(val, "%f", &delaySec); derr == nil {
										rules.CrawlDelay = time.Duration(delaySec * float64(time.Second))
									}
								}
							}
						}
					}
				}
			}
		}
	}

	robotsMu.Lock()
	robotsCache[host] = data
	robotsMu.Unlock()
	return data
}

func isAgentAllowed(targetURL string, agent string, robotsData *RobotsData) (bool, time.Duration) {
	parsed, err := url.Parse(targetURL)
	if err != nil {
		return false, 0
	}
	path := parsed.Path
	if path == "" {
		path = "/"
	}

	rules, exists := robotsData.Rules[agent]
	if !exists {
		return true, 0
	}

	longestAllow := -1
	for _, allow := range rules.AllowedPaths {
		if strings.HasPrefix(path, allow) {
			if len(allow) > longestAllow {
				longestAllow = len(allow)
			}
		}
	}

	longestDisallow := -1
	for _, disallow := range rules.DisallowedPaths {
		if strings.HasPrefix(path, disallow) {
			if len(disallow) > longestDisallow {
				longestDisallow = len(disallow)
			}
		}
	}

	if longestAllow > longestDisallow {
		return true, rules.CrawlDelay
	}
	if longestDisallow > longestAllow {
		return false, rules.CrawlDelay
	}

	return true, rules.CrawlDelay
}

func findAllowedAgent(targetURL string, robotsData *RobotsData) (string, time.Duration, bool) {
	// 1. If wildcard "*" is allowed, rotate realistic browser User-Agents!
	if allowed, delay := isAgentAllowed(targetURL, "*", robotsData); allowed {
		ua := rotatingUserAgents[rand.Intn(len(rotatingUserAgents))]
		return ua, delay, true
	}

	// 2. If wildcard is blocked but 'searqonbot' or 'searqon' is explicitly whitelisted, use it
	if allowed, delay := isAgentAllowed(targetURL, "searqonbot", robotsData); allowed {
		return "SearqonBot/1.0 (+https://sooriya04.github.io/Searqon/)", delay, true
	}
	if allowed, delay := isAgentAllowed(targetURL, "searqon", robotsData); allowed {
		return "SearqonBot/1.0 (+https://sooriya04.github.io/Searqon/)", delay, true
	}

	searchAgents := []string{"googlebot", "bingbot", "slurp", "duckduckbot", "baiduspider", "yandexbot"}
	for _, agent := range searchAgents {
		if _, exists := robotsData.Rules[agent]; exists {
			if allowed, delay := isAgentAllowed(targetURL, agent, robotsData); allowed {
				ua := agentUserAgents[agent]
				return ua, delay, true
			}
		}
	}

	for agent, ua := range agentUserAgents {
		if allowed, delay := isAgentAllowed(targetURL, agent, robotsData); allowed {
			return ua, delay, true
		}
	}

	return "", 0, false
}

func isAllowed(targetURL string, robotsData *RobotsData) bool {
	_, _, allowed := findAllowedAgent(targetURL, robotsData)
	return allowed
}
