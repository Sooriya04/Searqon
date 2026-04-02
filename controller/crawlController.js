/**
 * Searqon Crawl Controller
 *
 * Exposes Firecrawl-compatible endpoints:
 *   POST /api/scrape          → Single URL → markdown/text
 *   POST /api/map             → Discover all URLs on a domain
 *   POST /api/crawl           → Recursively crawl and scrape a site
 *   POST /api/extract         → Schema-based structured JSON extraction (LLM)
 *
 * All heavy lifting is done by the Go scraper (port 3002).
 * JS rendering falls back to Puppeteer for pages that fail static scraping.
 * Uses native fetch (Node 18+) — no axios.
 */

const { ScrapUrlBatch } = require('../scrapper/ScrapUrl');
const { searchDuckDuckGo } = require('../services/duckduckgo');

const GO_SCRAPER_URL   = 'http://127.0.0.1:3002';
const SCRAPER_TIMEOUT  = 30000; // ms

// ─── Helper: fetch JSON from Go scraper ───────────────────────────────────────

async function goPost(path, body, timeoutMs = SCRAPER_TIMEOUT) {
    const controller = new AbortController();
    const id = setTimeout(() => controller.abort(), timeoutMs);

    try {
        const res = await fetch(`${GO_SCRAPER_URL}${path}`, {
            method:  'POST',
            headers: { 'Content-Type': 'application/json' },
            body:    JSON.stringify(body),
            signal:  controller.signal
        });

        if (!res.ok) {
            const text = await res.text();
            throw new Error(`Go scraper ${path} → HTTP ${res.status}: ${text}`);
        }

        return await res.json();
    } finally {
        clearTimeout(id);
    }
}

// ─── Scrape ───────────────────────────────────────────────────────────────────

exports.scrape = async (req, res) => {
    try {
        const { url, format = 'markdown', useJs = false } = req.body;

        if (!url) {
            return res.status(400).json({ success: false, error: 'url is required' });
        }

        console.log(`[Crawl] Scrape: ${url} (format: ${format}, js: ${useJs})`);

        if (useJs) {
            return await handleJsScrape(url, format, res);
        }

        const data = await goPost('/scrape', { url, format });

        // Auto-fallback to Puppeteer if Go returns too little
        if (!data.error && data.wordCount < 30) {
            console.log(`[Crawl] Sparse content (${data.wordCount} words), trying JS fallback...`);
            return await handleJsScrape(url, format, res);
        }

        return res.json({
            success:   !data.error,
            url:       data.url,
            title:     data.title,
            content:   data.content,
            markdown:  data.markdown || null,
            wordCount: data.wordCount,
            duration:  data.duration,
            engine:    'go_static',
            error:     data.error || null
        });

    } catch (err) {
        console.error(`[Crawl] Scrape error:`, err.message);
        return res.status(500).json({ success: false, error: err.message });
    }
};

// ─── JS Rendering via Puppeteer ───────────────────────────────────────────────

async function handleJsScrape(url, format, res) {
    let browser = null;
    try {
        const puppeteer = require('puppeteer');
        browser = await puppeteer.launch({
            headless: 'new',
            args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-dev-shm-usage']
        });

        const page = await browser.newPage();

        // Block heavy resources to speed up load
        await page.setRequestInterception(true);
        page.on('request', r => {
            if (['image', 'stylesheet', 'font', 'media'].includes(r.resourceType())) {
                r.abort();
            } else {
                r.continue();
            }
        });

        await page.setUserAgent('Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/121.0.0.0 Safari/537.36');
        await page.goto(url, { waitUntil: 'domcontentloaded', timeout: 20000 });
        await new Promise(r => setTimeout(r, 1500));

        const title = await page.title();

        // Feed rendered URL back through Go for clean extraction
        const data = await goPost('/scrape', { url, format });

        return res.json({
            success:   true,
            url,
            title:     title || data.title,
            content:   data.content,
            markdown:  data.markdown || null,
            wordCount: data.wordCount,
            engine:    'puppeteer_js',
            error:     null
        });

    } catch (err) {
        console.error(`[Crawl] JS scrape error:`, err.message);
        return res.status(500).json({ success: false, error: `JS rendering failed: ${err.message}` });
    } finally {
        if (browser) await browser.close();
    }
}

// ─── Map ──────────────────────────────────────────────────────────────────────

exports.map = async (req, res) => {
    try {
        const { url, limit = 100, search } = req.body;

        if (!url) {
            return res.status(400).json({ success: false, error: 'url is required' });
        }

        console.log(`[Crawl] Map: ${url} (limit: ${limit})`);

        let { links, count, duration } = await goPost('/map', { url, limit });

        // Optional client-side filter by search term
        if (search && typeof search === 'string') {
            const term = search.toLowerCase();
            links = links.filter(l =>
                l.url.toLowerCase().includes(term) ||
                (l.title && l.title.toLowerCase().includes(term))
            );
            count = links.length;
        }

        return res.json({ success: true, sourceUrl: url, links, count, duration });

    } catch (err) {
        console.error(`[Crawl] Map error:`, err.message);
        return res.status(500).json({ success: false, error: err.message });
    }
};

// ─── Crawl ────────────────────────────────────────────────────────────────────

exports.crawl = async (req, res) => {
    try {
        const { url, limit = 30, depth = 2, format = 'markdown' } = req.body;

        if (!url) {
            return res.status(400).json({ success: false, error: 'url is required' });
        }

        const safeLimit = Math.min(limit, 50);
        const safeDepth = Math.min(depth, 3);

        console.log(`[Crawl] Crawl: ${url} (limit: ${safeLimit}, depth: ${safeDepth}, format: ${format})`);

        const { pages, total, duration } = await goPost(
            '/crawl',
            { url, limit: safeLimit, depth: safeDepth, format },
            120000 // crawls can take a while
        );

        const successPages = pages.filter(p => !p.error);
        const failedPages  = pages.filter(p =>  p.error);

        return res.json({
            success:   true,
            sourceUrl: url,
            status:    'completed',
            total,
            completed: successPages.length,
            failed:    failedPages.length,
            duration,
            data: successPages.map(p => ({
                url:       p.url,
                title:     p.title,
                markdown:  p.markdown || null,
                content:   p.content,
                wordCount: p.wordCount
            }))
        });

    } catch (err) {
        console.error(`[Crawl] Crawl error:`, err.message);
        return res.status(500).json({ success: false, error: err.message });
    }
};

// ─── Search + Scrape (Single Query) ─────────────────────────────────────────

exports.search = async (req, res) => {
    try {
        const { query, limit = 5 } = req.body;

        if (!query) {
            return res.status(400).json({ success: false, error: 'query is required' });
        }

        console.log(`[Crawl] Search (Firecrawl style): "${query}" (limit: ${limit})`);

        // Perform the search and scrape in one go using the DuckDuckGo service
        // (which we just updated to scrape all results with markdown)
        const results = await searchDuckDuckGo(query, limit);

        return res.json({
            success: true,
            query,
            totalResults: results.length,
            data: results.map(r => ({
                url: r.url,
                title: r.title,
                markdown: r.markdown,
                content: r.content,
                score: r.score,
                metadata: r.metadata
            }))
        });

    } catch (err) {
        console.error(`[Crawl] Search error:`, err.message);
        return res.status(500).json({ success: false, error: err.message });
    }
};

// ─── Extract (Structured JSON via LLM) ───────────────────────────────────────

exports.extract = async (req, res) => {
    try {
        const { url, urls, query, prompt, schema, limit = 3 } = req.body;

        if (!prompt) {
            return res.status(400).json({ success: false, error: 'prompt is required' });
        }

        let targetUrls = urls || (url ? [url] : null);

        // If no URLs are provided, but a query is, perform a search first
        if (!targetUrls && query) {
            console.log(`[Crawl] Extract-Search: Performing search for query: "${query}"`);
            const searchResults = await searchDuckDuckGo(query, limit);
            targetUrls = searchResults.map(r => r.url);
        }

        if (!targetUrls || targetUrls.length === 0) {
            return res.status(400).json({ success: false, error: 'url, urls, or query is required' });
        }

        console.log(`[Crawl] Extract: ${targetUrls.length} URL(s) — "${prompt.slice(0, 60)}..."`);

        // Scrape all target URLs
        const scraped   = await ScrapUrlBatch(targetUrls, { format: 'markdown' });
        const validDocs = scraped.filter(d => d && !d.error && d.wordCount > 10);

        if (validDocs.length === 0) {
            return res.status(422).json({ success: false, error: 'Could not extract content from any URL' });
        }

        // Build context + schema instructions
        const context = validDocs.map(d =>
            `## ${d.title || d.url}\nSource: ${d.url}\n\n${d.markdown || d.content}`
        ).join('\n\n---\n\n');

        const schemaInstruction = schema
            ? `\n\nRespond ONLY with valid JSON matching this schema:\n${JSON.stringify(schema, null, 2)}`
            : '\n\nRespond in structured JSON.';

        const fullPrompt = `${prompt}${schemaInstruction}\n\n---\n\nContext from web:\n${context.slice(0, 12000)}`;

        const extractionService = require('../services/extractionService');
        const extracted = await extractionService.extract(fullPrompt);

        return res.json({
            success: true,
            sources: validDocs.map(d => d.url),
            data:    extracted
        });

    } catch (err) {
        console.error(`[Crawl] Extract error:`, err.message);
        return res.status(500).json({ success: false, error: err.message });
    }
};
