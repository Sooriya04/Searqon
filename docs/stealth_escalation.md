# Smart Stealth & Anti-Bot Escalation Architecture

Searqon features an automated, multi-tier **Smart Stealth & Anti-Bot Escalation Ladder**. Rather than paying the latency and resource penalty of spinning up headless browser subprocesses or routing through proxy pools for every request, Searqon starts with lightweight native execution and dynamically escalates to stealth personas, headless browsers, and residential proxies only when bot protection or JavaScript gates are detected.

---

## Architectural Diagram

![Smart Stealth & Anti-Bot Escalation Ladder](./images/stealth_escalation.svg)

---

## The 4-Tier Escalation Ladder

The engine processes target URLs sequentially through four distinct tiers:

### Tier 1: Fast Direct HTTP (~50–150ms)
* **Goal:** Maximum throughput, sub-100ms response times for the majority of unblocked web content.
* **Mechanics:** Direct, unproxied native Go HTTP client connection using connection pooling, HTTP/2 multiplexing, and Brotli/Gzip decompression.
* **Headers:** Lightweight, standard modern desktop headers (`Accept`, `Accept-Language`, `Accept-Encoding`).
* **Timeout:** 2500ms.
* **Success Condition:** HTTP status `< 400`, word count $\ge 20$, and zero anti-bot challenge signatures detected.
* **Escalation Trigger:** Encountering HTTP `403`, `429`, `503`, connection resets, or challenge signatures triggers immediate escalation to Tier 2.

### Tier 2: Anti-Bot Spoofed Headers & TLS 1.3 Fingerprinting
* **Goal:** Bypass 80%+ of automated bot blockers and WAF filters with zero browser subprocess overhead.
* **Mechanics:** Uses coherent browser personas matching exact modern browser fingerprints:
  * **Chrome 128 (Windows 11 / macOS Sonoma / Linux):** Sends matching Client Hints (`Sec-CH-UA`, `Sec-CH-UA-Mobile: ?0`, `Sec-CH-UA-Platform`), `Sec-Fetch-Dest: document`, `Sec-Fetch-Mode: navigate`, `Sec-Fetch-Site: cross-site`, `Priority: u=0, i`.
  * **Safari 17.6 (macOS):** Correctly omits `Sec-CH-UA` headers to match genuine WebKit behavior.
  * **Firefox 129 (Windows 11):** Configured with Gecko-specific accept headers and omitted Client Hints.
  * **TLS 1.3 Emulation:** Enforces ALPN `["h2", "http/1.1"]`, modern elliptic curves (`X25519`, `P-256`, `P-384`), and browser cipher suites.
  * **Referer Spoofing:** Injects high-authority search referers (Google, DuckDuckGo, Bing) or root domain origins.
* **Timeout:** 4000ms.
* **Success Condition:** HTTP status `< 400`, word count $\ge 20$, challenge bypassed.
* **Escalation Trigger:** Detecting JavaScript requirements (empty client-side SPA shells), Cloudflare Turnstile interactive challenges, or continued 403 blocks escalates to Tier 3.

### Tier 3: Headless Browser Engine (Camoufox / Lightpanda)
* **Goal:** Execute client-side JavaScript, hydrate dynamic Single Page Applications (React, Next.js, Vue), and pass interactive browser challenges.
* **Supported Engines:**
  * **[Camoufox](https://github.com/daijro/camoufox):** C++ modified anti-detect browser based on Firefox designed specifically to defeat Cloudflare Turnstile, DataDome, and advanced bot detection systems.
  * **[Lightpanda](https://github.com/lightpanda-io/browser):** High-performance headless browser built in C/WebAssembly optimized for AI scraping.
* **Timeout:** 6000ms–8000ms.
* **Success Condition:** Live DOM rendered, hydrated content extracted, and challenge solved.
* **Escalation Trigger:** If headless browsers are unavailable, or if the server enforces hard IP-level rate-limiting or datacenter subnet bans, the request escalates to Tier 4.

### Tier 4: Residential / Rotating Proxy Fallback
* **Goal:** Bypass hard IP-level blocks, rate limits, and regional geofencing.
* **Mechanics:** Routes the request through the configured residential proxy pool using spoofed browser headers or launches the headless browser engine with the `--proxy` parameter.
* **Proxy Pools:**
  * Dedicated Residential Proxies (`RESIDENTIAL_PROXY_URL`, `RESIDENTIAL_PROXIES`, or `residential_proxies.txt`).
  * Rotating Datacenter Proxies (`ROTATING_PROXIES` or `proxies.txt`).
* **Timeout:** 6000ms.

---

## Anti-Bot Detection Matrix

Searqon inspects every response in real time using the [`DetectBotChallenge`](file:///home/sooriya/Documents/Searqon/src/scraper/bot_detection.go) detector before accepting content:

| Provider / Type | Detected Signatures & Markers | Action |
|---|---|---|
| **Cloudflare** | `<title>Just a moment...</title>`, `cf-browser-verification`, `challenge-platform`, `cf-turnstile`, `cf_chl_`, `Attention Required! \| Cloudflare`, `cf-mitigated` | Escalate Tier 1 → 2 → 3 |
| **DataDome** | `geo.captcha-delivery.com`, `datadome.js`, `protected by datadome` | Escalate to Camoufox / Proxy |
| **PerimeterX / HUMAN** | `px-captcha`, `_pxAppId`, `access to this page has been denied because we believe you are using automation tools` | Escalate to Camoufox / Proxy |
| **Arkose Labs / Kasada** | `arkoselabs`, `client-api.arkoselabs.com`, `kasada`, `distil_captcha` | Escalate to Headless Browser |
| **Imperva / Incapsula** | `incapsula incident id`, `_incapsula_resource` | Escalate to Headless Browser |
| **Generic Captchas** | `verify you are human`, `bot detection`, `unusual traffic from your computer network`, `g-recaptcha`, `hcaptcha` | Escalate |
| **Empty JS SPA Gates** | Content length < 500 bytes with `<div id="root"></div>` and missing text body | Escalate to Headless Browser |
| **Rate Limiting (429)** | HTTP 429 Too Many Requests | Escalate to Residential Proxy |

---

## Configuration

Set the desired options in your `.env` file:

```env
# Headless Browser Engines (Tier 3)
LIGHTPANDA_ENABLE=true
LIGHTPANDA_BINARY_PATH=/usr/local/bin/lightpanda

CAMOUFOX_ENABLE=true
CAMOUFOX_PATH=/usr/local/bin/camoufox

# Anti-Bot & Residential Proxy Controls (Tier 4)
ROTATING_PROXIES=http://proxy1:8080,http://proxy2:8080
RESIDENTIAL_PROXY_URL=http://username:password@residential.provider.com:8000
RESIDENTIAL_PROXIES=http://user:pass@res1:8000,http://user:pass@res2:8000
```

Alternatively, place proxy lists into `residential_proxies.txt` or `proxies.txt` in the root folder (one proxy per line).

---

## API Usage & Overrides

### Automatic Escalation (Default)
By default, all calls to `POST /scrape`, `POST /scrape/batch`, and `POST /search` execute the full 4-tier escalation ladder automatically:

```bash
curl -X POST http://localhost:4001/scrape \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com/protected-page"}'
```

### Manual Stealth Level Override
You can pin a specific tier using the `stealth_level` parameter:

```bash
# Force fast native HTTP only (skips escalation)
curl -X POST http://localhost:4001/scrape \
  -H "Content-Type: application/json" \
  -d '{"url": "https://en.wikipedia.org/wiki/Go", "stealth_level": "fast_http"}'

# Jump directly to Anti-Bot Spoofed Headers
curl -X POST http://localhost:4001/scrape \
  -H "Content-Type: application/json" \
  -d '{"url": "https://example.com", "stealth_level": "spoofed_headers"}'

# Force Headless Browser execution directly
curl -X POST http://localhost:4001/scrape \
  -H "Content-Type: application/json" \
  -d '{"url": "https://app.spa-website.com", "stealth_level": "headless"}'

# Route immediately through Residential Proxy
curl -X POST http://localhost:4001/scrape \
  -H "Content-Type: application/json" \
  -d '{"url": "https://rate-limited-domain.com", "stealth_level": "proxy"}'
```

Supported `stealth_level` values:
* `"auto"` (default — progressive 4-tier escalation)
* `"fast_http"` (Tier 1 only)
* `"spoofed_headers"` (Starts at Tier 2)
* `"headless"` or `"browser"` (Starts at Tier 3)
* `"proxy"` (Starts at Tier 4)

---

## Response Telemetry

Every scrape response includes metadata detailing which tier resolved the page:

```json
{
  "url": "https://example.com/protected-article",
  "title": "Deep Web Intelligence",
  "content": "...",
  "markdown": "# Deep Web Intelligence\n\n...",
  "statusCode": 200,
  "scraped": true,
  "escalation_tier": "spoofed_headers",
  "bot_detected": true,
  "render_method": "go",
  "duration": 142,
  "fetchDurationMs": 138,
  "cached": false
}
```

* `escalation_tier`: The tier that successfully resolved the page (`fast_http`, `spoofed_headers`, `headless_browser`, `residential_proxy`).
* `bot_detected`: Boolean flag indicating whether an anti-bot challenge or WAF gate had to be bypassed.
* `render_method`: The rendering engine used (`go`, `camoufox`, or `lightpanda`).
