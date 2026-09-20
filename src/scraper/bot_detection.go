package scraper

import (
	"strings"

	"github.com/PuerkitoBio/goquery"
	"src/utils"
)

// BotBlockReason details why a page was detected as a bot challenge or blocked.
type BotBlockReason struct {
	IsBlocked bool   `json:"is_blocked"`
	Category  string `json:"category,omitempty"` // "http_status", "cloudflare", "datadome", "perimeterx", "captcha", "js_gate", "waf"
	Details   string `json:"details,omitempty"`
}

// challengeSignatures maps detection categories to characteristic lowercase string signatures.
var challengeSignatures = []struct {
	category string
	signals  []string
}{
	{
		category: "cloudflare",
		signals: []string{
			"<title>just a moment...</title>",
			"cf-browser-verification",
			"challenge-platform",
			"cf-turnstile",
			"cf_chl_",
			"cf-chl-",
			"attention required! | cloudflare",
			"checking your browser before accessing",
			"why have i been blocked?",
			"cloudflare ray id",
			"ray id:",
			"cf-mitigated",
			"cf-wrapper",
		},
	},
	{
		category: "datadome",
		signals: []string{
			"datadome",
			"geo.captcha-delivery.com",
			"datadome.js",
			"protected by datadome",
		},
	},
	{
		category: "perimeterx",
		signals: []string{
			"perimeterx",
			"px-captcha",
			"_pxappid",
			"human security",
			"access to this page has been denied because we believe you are using automation tools",
		},
	},
	{
		category: "arkose_kasada_imperva",
		signals: []string{
			"arkoselabs",
			"client-api.arkoselabs.com",
			"kasada",
			"distil_captcha",
			"incapsula incident id",
			"_incapsula_resource",
			"imperva",
		},
	},
	{
		category: "captcha",
		signals: []string{
			"verify you are human",
			"verify you're a human",
			"security check to access",
			"press and hold",
			"bot detection",
			"robot or human?",
			"unusual traffic from your computer network",
			"g-recaptcha",
			"hcaptcha",
		},
	},
	{
		category: "waf",
		signals: []string{
			"access denied",
			"403 forbidden",
			"request blocked",
			"web application firewall",
			"security policy violation",
			"you don't have permission to access",
		},
	},
}

// DetectBotChallenge checks HTTP status code, body, and content-type for anti-bot blocks or challenges.
func DetectBotChallenge(statusCode int, body string, contentType string) BotBlockReason {
	lowerBody := strings.ToLower(body)

	// 1. Check HTTP Status Codes indicating automated gatekeeping
	if statusCode == 429 {
		return BotBlockReason{
			IsBlocked: true,
			Category:  "rate_limit",
			Details:   "HTTP 429 Too Many Requests",
		}
	}

	if statusCode == 503 && (strings.Contains(lowerBody, "cloudflare") || strings.Contains(lowerBody, "just a moment") || strings.Contains(lowerBody, "challenge")) {
		return BotBlockReason{
			IsBlocked: true,
			Category:  "cloudflare",
			Details:   "HTTP 503 Cloudflare Under Attack challenge page",
		}
	}

	// 2. Check content signatures across known anti-bot providers
	for _, group := range challengeSignatures {
		for _, signal := range group.signals {
			if strings.Contains(lowerBody, signal) {
				// Special guard for WAF / generic signals: ensure page is actually small or blocked
				if group.category == "waf" {
					if statusCode == 403 || utils.CountWords(body) < 60 {
						return BotBlockReason{
							IsBlocked: true,
							Category:  group.category,
							Details:   "Anti-bot block signature detected: " + signal,
						}
					}
					continue
				}

				return BotBlockReason{
					IsBlocked: true,
					Category:  group.category,
					Details:   "Anti-bot challenge signature detected: " + signal,
				}
			}
		}
	}

	// 3. Status 403 with standard HTML
	if statusCode == 403 {
		return BotBlockReason{
			IsBlocked: true,
			Category:  "http_status",
			Details:   "HTTP 403 Forbidden",
		}
	}

	// 4. Check for client-side JavaScript gates and empty SPA shells (Angular, React, Vue, Next.js)
	if strings.Contains(contentType, "text/html") || contentType == "" {
		if !hasSubstantialText(body) {
			if strings.Contains(lowerBody, "<app-root") ||
				strings.Contains(lowerBody, `id="root"`) ||
				strings.Contains(lowerBody, `id="app"`) ||
				strings.Contains(lowerBody, `id="__next"`) ||
				strings.Contains(lowerBody, "window.prerenderready = false") ||
				strings.Contains(lowerBody, "prerenderready=false") ||
				(strings.Contains(lowerBody, "javascript") && (strings.Contains(lowerBody, "enable") || strings.Contains(lowerBody, "required") || strings.Contains(lowerBody, "disabled"))) {
				return BotBlockReason{
					IsBlocked: true,
					Category:  "js_gate",
					Details:   "Client-side Single Page Application (SPA) shell requiring JavaScript execution",
				}
			}
		}
	}

	return BotBlockReason{
		IsBlocked: false,
	}
}

func hasSubstantialText(body string) bool {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(body))
	if err != nil || doc == nil {
		return utils.CountWords(body) >= 25
	}
	doc.Find("script, style, noscript, svg").Remove()
	text := utils.CleanText(doc.Find("body").Text())
	return utils.CountWords(text) >= 25
}
