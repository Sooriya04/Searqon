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

// ─── Production Job Store (In-memory) ──────────────────────────────────────────
// In a real-world production app, this would be Redis or a database.
const jobs = new Map();

function createJob(type, payload = {}) {
    const id = Date.now().toString(36) + Math.random().toString(36).substring(2, 5);
    const job = {
        id,
        type,
        status: 'pending',
        progress: 0,
        createdAt: new Date(),
        updatedAt: new Date(),
        data: null,
        error: null,
        payload
    };
    jobs.set(id, job);
    return job;
}

function updateJob(id, updates) {
    const job = jobs.get(id);
    if (job) {
        Object.assign(job, updates, { updatedAt: new Date() });
    }
}

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
        const { url, limit = 30, depth = 2, format = 'markdown', webhook } = req.body;

        if (!url) {
            return res.status(400).json({ success: false, error: 'url is required' });
        }

        const safeLimit = Math.min(limit, 100);
        const safeDepth = Math.min(depth, 5);

        console.log(`[Crawl] Job Created: ${url} (limit: ${safeLimit}, depth: ${safeDepth})`);

        const job = createJob('crawl', { url, limit: safeLimit, depth: safeDepth, format });

        // Start the crawl in the background (Async)
        process.nextTick(async () => {
            try {
                updateJob(job.id, { status: 'running' });
                
                const response = await goPost(
                    '/crawl',
                    { url, limit: safeLimit, depth: safeDepth, format },
                    300000 // 5m timeout for batch jobs
                );

                const successPages = response.pages.filter(p => !p.error);
                const result = {
                    success:   true,
                    sourceUrl: url,
                    status:    'completed',
                    total:     response.total,
                    completed: successPages.length,
                    failed:    response.pages.length - successPages.length,
                    duration:  response.duration,
                    data:      successPages.map(p => ({
                        url:       p.url,
                        title:     p.title,
                        markdown:  p.markdown || null,
                        content:   p.content,
                        wordCount: p.wordCount,
                        metadata:  p.metadata
                    }))
                };

                updateJob(job.id, { status: 'completed', data: result, progress: 100 });

                // If webhook is provided, trigger it
                if (webhook) {
                    console.log(`[Crawl] Triggering webhook for job ${job.id}: ${webhook}`);
                    fetch(webhook, {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify(result)
                    }).catch(e => console.error(`[Crawl] Webhook failed:`, e.message));
                }

            } catch (err) {
                console.error(`[Crawl] Job ${job.id} failed:`, err.message);
                updateJob(job.id, { status: 'failed', error: err.message });
            }
        });

        // Return the Job ID immediately (Production standard)
        return res.status(202).json({
            success: true,
            jobId: job.id,
            statusUrl: `/api/crawl/${job.id}`
        });

    } catch (err) {
        console.error(`[Crawl] Crawl initiation error:`, err.message);
        return res.status(500).json({ success: false, error: err.message });
    }
};

// ─── Get Crawl Status (Job Status Polling) ───────────────────────────────────

exports.getCrawlStatus = async (req, res) => {
    const { id } = req.params;
    const job = jobs.get(id);

    if (!job) {
        return res.status(404).json({ success: false, error: 'Job not found' });
    }

    // Standard Firecrawl-style response object
    const response = {
        success:  true,
        status:   job.status,
        progress: job.progress,
        data:     job.data || null,
        error:    job.error || null,
        total:    job.data ? job.data.total : 0,
        completed: job.data ? job.data.completed : 0
    };

    return res.json(response);
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

        let effectivePrompt = prompt;

        if (!effectivePrompt && query) {
            effectivePrompt = query;
        }

        if (!effectivePrompt) {
            return res.status(400).json({ success: false, error: 'prompt or query is required' });
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

        console.log(`[Crawl] Extract: ${targetUrls.length} URL(s) — "${effectivePrompt.slice(0, 60)}..."`);

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
            : '\n\nRespond in clean, valid JSON object format.';

        const strictRules = '\n\nCRITICAL: Do not include conversational text or markdown code blocks outside the JSON. Do NOT use thousands separators in numbers (e.g., use 18000000 instead of 18,000,000). Your response must be parseable by JSON.parse().';

        const fullPrompt = `${effectivePrompt}${schemaInstruction}${strictRules}\n\n---\n\nContext from web:\n${context.slice(0, 12000)}`;

        const extractionService = require('../services/extractionService');
        const extracted = await extractionService.extract(fullPrompt);

        return res.json({
            success: true,
            data:    extracted,
            metadata: {
                sources: validDocs.map(d => ({
                    url: d.url,
                    title: d.title,
                    wordCount: d.wordCount
                })),
                engine: extractionService.BACKEND || 'ollama'
            }
        });

    } catch (err) {
        console.error(`[Crawl] Extract error:`, err.message);
        return res.status(500).json({ success: false, error: err.message });
    }
};
