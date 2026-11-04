# Searqon Embedded Engine Architecture (Zero-Docker Python SDK)

> **Proposal & Architecture Document:** Compiling Searqon's Go Core into an Embedded, Zero-Dependency Python Package.

---

## 💡 Executive Summary

Currently, developers run Searqon as a standalone container or local background process (`go run .` or `docker run -p 7493:7493 sooriya04/searqon:latest`).

While this containerized approach is ideal for production servers and microservice clusters, **Python AI agent builders (LangChain, LlamaIndex, CrewAI) prefer zero-setup local packages**:

```bash
pip install searqon
```

```python
import searqon

# ZERO Docker. ZERO manual server setup.
# The Go engine runs directly under the hood!
result = searqon.scrape("https://news.ycombinator.com")
print(result.markdown)
```

By compiling the Go core into a self-contained binary or shared library bundled directly inside the Python wheel, developers get the **raw performance of Go** (concurrency, TLS fingerprint spoofing, minimal RAM) with the **dead-simple developer experience of Python**.

---

## 🛠️ Implementation Approaches

There are two primary ways to bundle Go into a Python package, mirroring how modern developer tools (**`esbuild`**, **`playwright`**, and **`ruff`**) operate:

### Approach A: The "Self-Contained Wheel" (Recommended)
*Used by `playwright`, `esbuild-python`, `prisma-client-py`, `uv`*

In this model, the Python package bundles a precompiled, statically linked Go binary directly inside its distribution wheel:

```text
searqon/
├── pyproject.toml
├── searqon/
│   ├── __init__.py          # Exposes search(), scrape(), map(), extract()
│   ├── client.py            # HTTP & IPC client
│   ├── engine.py            # Automatic child-process lifecycle manager
│   └── bin/
│       ├── searqon-linux-amd64
│       ├── searqon-darwin-arm64
│       └── searqon-windows-amd64.exe
```

#### How it works under the hood:
1. When Python imports `searqon`, the `engine.py` manager checks if a local Searqon instance is already reachable on `:7493` or a user-provided `base_url`.
2. If no server is running, Python silently spawns the bundled static binary (`searqon-linux-amd64`) on an ephemeral local port or domain socket with flags:
   `--no-searxng --db=sqlite --redis=false`
3. The engine initializes in **~15 milliseconds** with an 8 MB RAM footprint.
4. Python communicates with the engine via zero-latency local HTTP or stdin/stdout JSON-RPC.
5. When Python terminates, an `atexit` hook terminates the background process cleanly.

#### Advantages:
* **Zero CGO / ABI Complications:** Works across all Linux distributions (glibc and musl/Alpine), macOS, and Windows.
* **Dual-Mode:** Can seamlessly switch between embedded local mode and remote production servers (`Searqon(base_url="https://remote.server:7493")`).
* **Easy Updates:** The Go binary is compiled once via GitHub Actions release pipelines and attached to PyPI wheels.

---

### Approach B: C-Shared Library (`-buildmode=c-shared`) via `ctypes`
*Used by high-throughput native extensions*

Go provides a built-in compilation target to create native shared libraries with C headers:

```bash
go build -buildmode=c-shared -o libsearqon.so src/main.go
```

In Python, the C-shared library is loaded directly into the interpreter's process space using `ctypes` or `cffi`:

```python
import ctypes
import json

lib = ctypes.CDLL("./libsearqon.so")

# Configure return types
lib.ScrapeURL.restype = ctypes.c_char_p
lib.ScrapeURL.argtypes = [ctypes.c_char_p]

# Execute in-process
raw_json = lib.ScrapeURL(b"https://news.ycombinator.com")
data = json.loads(raw_json.decode("utf-8"))
```

#### Advantages:
* **True In-Memory Execution:** Zero network stack, zero socket overhead.
* **Single Process:** Everything lives inside Python's PID.

#### Trade-offs:
* Cross-compiling CGO wheels across multiple OS versions and libc variants requires a `cibuildwheel` matrix in CI.

---

## 📦 What Works 100% Standalone (No Docker Required)

Even without external services (such as SearXNG or Redis), the compiled Go engine provides complete functionality:

| Feature | Standalone Status | Implementation |
| :--- | :---: | :--- |
| **4-Tier Scraper** | ✅ **100% Standalone** | Fast HTTP + Spoofed TLS 1.3 Personas (Chrome 128, Safari 17) |
| **Markdown Extraction** | ✅ **100% Standalone** | Pure Go HTML sanitization, stripping scripts, styles, and ads |
| **Domain Crawler / Sitemap Mapper** | ✅ **100% Standalone** | Recursive link extraction and domain boundary filtering (`/map`) |
| **Robots.txt Compliance** | ✅ **100% Standalone** | Native parser with crawl-delay and disallow enforcement |
| **Direct Meta-Search** | ✅ **100% Standalone** | Native multi-provider search (DuckDuckGo, Wikipedia, ArXiv) |
| **Structured JSON Extraction** | ✅ **100% Standalone** | CSS selector and schema parsing |
| **Token & Word Counters** | ✅ **100% Standalone** | Accurate LLM token cost estimations |
| **SearXNG Aggregation** | ⚠️ Optional | Used if `SEARXNG_URL` is set; falls back to direct search if absent |
| **Redis L1 Cache** | ⚠️ Optional | Used if Redis host is reachable; falls back to in-memory/SQLite |

---

## 🚀 Developer Experience Comparison

### Traditional Setup (Microservice):
```bash
# 1. Install Docker
# 2. Pull container
docker pull sooriya04/searqon:latest
# 3. Run container
docker run -d -p 7493:7493 sooriya04/searqon:latest
# 4. In Python, write HTTP requests
pip install requests
```

### Embedded Engine Setup (Zero-Docker):
```bash
pip install searqon
```
```python
import searqon

results = searqon.search("transformer attention mechanisms", limit=5)
page = searqon.scrape("https://arxiv.org/abs/1706.03762")

print(page.markdown)
```

---

## 🗺️ Roadmap & Next Steps

1. **Step 1:** Build the core Python client library (`sdks/python/searqon/`) supporting standard REST connections.
2. **Step 2:** Add the `engine.py` lifecycle manager to discover and spawn local binaries.
3. **Step 3:** Configure GitHub Actions matrix in `.github/workflows/ci.yml` to compile static binaries (`amd64`, `arm64`) and bundle them into PyPI wheel distributions.
