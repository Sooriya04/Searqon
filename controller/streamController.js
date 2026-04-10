const { searchArxiv } = require("../services/arxiv");
const { searchDOAJ } = require("../services/doaj");
const { searchMedRxiv } = require("../services/medrxiv");
const { searchDuckDuckGo } = require("../services/duckduckgo");
const { searchOpenAlex } = require("../services/openalex");
const { searchPubMed } = require("../services/pubmed");
const { searchHNByQuery } = require("../services/hackernews");
const { wikiSearch } = require("../services/wiki");
const { searchWithReadmes } = require("../services/github");
const { reddit } = require("../services/reddit");
const { searchWeb } = require("../services/web");
const { searchGeeksForGeeks } = require("../services/geeksforgeeks");
const { searchYoutube } = require("../services/youtube");
const { routeQuery } = require("../services/classifierService");

// ─── Source Registry ──────────────────────────────────────────────────────────

const ALL_SOURCES = [
    { name: "arxiv", fn: (q, l) => searchArxiv(q, l) },
    { name: "doaj", fn: (q, l) => searchDOAJ(q, l) },
    { name: "medrxiv", fn: (q, l) => searchMedRxiv(q, l) },
    { name: "openalex", fn: (q, l) => searchOpenAlex(q, l) },
    { name: "pubmed", fn: (q, l) => searchPubMed(q, l) },
    { name: "hackernews", fn: (q, l) => searchHNByQuery(q, l) },
    { name: "wikipedia", fn: (q) => wikiSearch(q) },
    { name: "github", fn: (q, l) => searchWithReadmes(q, l) },
    { name: "reddit", fn: (q, l) => reddit(q, l) },
    { name: "geeksforgeeks", fn: (q, l) => searchGeeksForGeeks(q, l) },
    { name: "youtube", fn: (q, l) => searchYoutube(q, l) },
    { name: "web", fn: (q, l) => searchWeb(q, l) },
    { name: "duckduckgo", fn: (q, l) => searchDuckDuckGo(q, l) },
];

const SOURCE_MAP = Object.fromEntries(ALL_SOURCES.map((s) => [s.name, s]));

// ─── Helpers ──────────────────────────────────────────────────────────────────

function normalizeResults(data) {
    if (!data) return [];
    if (data.results && Array.isArray(data.results)) return data.results;
    if (Array.isArray(data)) return data;
    return [data];
}

async function runSource(name, query, limit) {
    const def = SOURCE_MAP[name];
    if (!def) return { name, status: "skipped", error: `Unknown source: ${name}` };
    const SOURCE_TIMEOUT_MS = 10000;
    try {
        const sourcePromise = def.fn(query, limit);
        const timeoutPromise = new Promise((_, reject) =>
            setTimeout(() => reject(new Error("Source timeout")), SOURCE_TIMEOUT_MS)
        );
        const raw = await Promise.race([sourcePromise, timeoutPromise]);
        const data = normalizeResults(raw);
        return { name, status: "ok", count: data.length, data };
    } catch (err) {
        const msg = err.message === "Source timeout" ? "timed out after 10s" : err.message;
        return { name, status: "failed", error: msg || "Unknown error" };
    }
}

// ─── Controller ───────────────────────────────────────────────────────────────

exports.streamSearchGet = async (req, res) => {
    try {
        const { query, limit } = req.query;
        if (!query || typeof query !== "string" || query.trim().length === 0) {
            return res.status(400).json({ error: "query must be a non-empty string" });
        }

        const maxResults = limit ? parseInt(limit, 10) : 5;
        const startTime = Date.now();

        // 1. SSE Headers
        res.setHeader('Content-Type', 'text/event-stream');
        res.setHeader('Cache-Control', 'no-cache');
        res.setHeader('Connection', 'keep-alive');
        res.write(`data: ${JSON.stringify({ type: 'status', data: 'Routing Query...' })}\n\n`);

        // 2. Fetch Routing & sources
        const routing = await routeQuery(query);
        const domainSourceNames = routing.sources.filter((s) => s !== "duckduckgo");
        const hasBaseline = routing.sources.includes("duckduckgo");

        res.write(`data: ${JSON.stringify({ type: 'status', data: 'Retrieving sources...' })}\n\n`);

        // 3. Execution
        const phase1Results = await Promise.all(
            domainSourceNames.map((name) => runSource(name, query, maxResults))
        );

        let ddgResult = null;
        if (hasBaseline) {
            ddgResult = await runSource("duckduckgo", query, maxResults);
        }

        // 4. Send Results
        const sources = {};
        for (const r of phase1Results) {
            sources[r.name] = r.status === "ok" ? { status: "ok", count: r.count, data: r.data } : { status: r.status, error: r.error };
        }
        if (ddgResult) {
            sources["duckduckgo"] = ddgResult.status === "ok" ? { status: "ok", count: ddgResult.count, data: ddgResult.data } : { status: ddgResult.status, error: ddgResult.error };
        }

        res.write(`data: ${JSON.stringify({ type: 'sources', data: sources })}\n\n`);

        const duration = Date.now() - startTime;
        res.write(`data: ${JSON.stringify({ type: 'done', data: { duration: `${duration}ms` } })}\n\n`);
        res.end();

    } catch (error) {
        res.write(`data: ${JSON.stringify({ type: 'error', data: error.message })}\n\n`);
        res.end();
    }
};
