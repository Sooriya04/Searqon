# Installation Guide

---

## Prerequisites

| Requirement | Version | Notes |
|---|---|---|
| Go | 1.21+ | [Download](https://go.dev/dl/) |
| Git | any | |
| Docker | optional | Only needed for SearXNG or containerised deployment |

Check your Go version:
```bash
go version
# go version go1.24.0 linux/amd64
```

---

## 1. Clone the Repository

```bash
git clone https://github.com/Sooriya04/Searqon
cd searqon
```

---

## 2. Install Dependencies

Go modules handle all dependencies automatically:

```bash
cd go_scraper
go mod download
```

Dependencies pulled:
- `github.com/PuerkitoBio/goquery` — HTML parsing
- `github.com/go-shiori/go-readability` — Article extraction
- `github.com/JohannesKaufmann/html-to-markdown/v2` — HTML → Markdown
- `github.com/andybalholm/brotli` — Brotli decompression

---

## 3. Run the Server

### Development (no build step)
```bash
cd go_scraper
go run .
```

### Using Makefile
```bash
make run       # go run . (dev mode)
make build     # compile to ./searqon binary
make start     # build + run binary
```

You should see:
```
[Searqon] Starting on :4001
[Searqon] Endpoints: POST /search, POST /scrape, POST /scrape/batch ...
```

---

## 4. Verify It Works

```bash
curl http://localhost:4001/health
# {"status":"ok","engine":"go_scraper",...}
```

Run a real search:
```bash
curl -X POST http://localhost:4001/search \
  -H "Content-Type: application/json" \
  -d '{"query": "golang concurrency", "limit": 3, "scrape": false}'
```

---

## 5. (Optional) Run SearXNG for Better Results

SearXNG is a self-hosted metasearch engine that significantly improves result quality and removes rate-limiting. Searqon automatically detects it at `http://localhost:8080`.

```bash
# Start SearXNG as a separate container
docker run -d --name searxng -p 8080:8080 searxng/searxng

# Or use the Makefile shortcut
make searxng
```

> **Note:** SearXNG and Searqon are completely independent microservices.  
> Searqon auto-falls back to DuckDuckGo if SearXNG is offline.

---

## Updating

```bash
git pull
cd go_scraper
go mod download
make build
```
