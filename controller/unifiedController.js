/**
 * Searqon Unified Search Controller
 *
 * Flow:
 *  1. Classify query → get ordered source list (Semantic Intent Engine)
 *  2. Execute domain-specific sources concurrently (phase 1)
 *  3. Execute DuckDuckGo last as the baseline (phase 2)
 *  4. Summarize all results using TF-IDF extractive highlights (phase 3)
 *  5. Return merged results with routing metadata + research highlights
 */

const { searchArxiv }        = require("../services/arxiv");
const { searchDOAJ }         = require("../services/doaj");
const { searchMedRxiv }      = require("../services/medrxiv");
const { searchDuckDuckGo }   = require("../services/duckduckgo");
const { searchOpenAlex }     = require("../services/openalex");
const { searchPubMed }       = require("../services/pubmed");
const { searchHNByQuery }    = require("../services/hackernews");
const { wikiSearch }         = require("../services/wiki");
const { searchWithReadmes }  = require("../services/github");
const { reddit }             = require("../services/reddit");
const { searchWeb }          = require("../services/web");
const { searchGeeksForGeeks} = require("../services/geeksforgeeks");
const { searchYoutube }      = require("../services/youtube");
const { routeQuery, summarizeResults } = require("../services/classifierService");

// ─── Source Registry ──────────────────────────────────────────────────────────

const ALL_SOURCES = [
    { name: "arxiv",         fn: (q, l) => searchArxiv(q, l)         },
    { name: "doaj",          fn: (q, l) => searchDOAJ(q, l)          },
    { name: "medrxiv",       fn: (q, l) => searchMedRxiv(q, l)       },
    { name: "openalex",      fn: (q, l) => searchOpenAlex(q, l)      },
    { name: "pubmed",        fn: (q, l) => searchPubMed(q, l)        },
    { name: "hackernews",    fn: (q, l) => searchHNByQuery(q, l)     },
    { name: "wikipedia",     fn: (q)    => wikiSearch(q)             },
    { name: "github",        fn: (q, l) => searchWithReadmes(q, l)   },
    { name: "reddit",        fn: (q, l) => reddit(q, l)              },
    { name: "geeksforgeeks", fn: (q, l) => searchGeeksForGeeks(q, l) },
    { name: "youtube",       fn: (q, l) => searchYoutube(q, l)       },
    { name: "web",           fn: (q, l) => searchWeb(q, l)           },
    { name: "duckduckgo",    fn: (q, l) => searchDuckDuckGo(q, l)    },
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
        console.error(`[Unified] ${name} failed: ${msg}`);
        return { name, status: "failed", error: msg || "Unknown error" };
    }
}

/**
 * Flatten all successful source results into a single array of documents
 * suitable for the TF-IDF summarizer.
 */
function flattenForSummarizer(sources) {
    const docs = [];
    for (const [sourceName, sourceData] of Object.entries(sources)) {
        if (sourceData.status !== "ok" || !sourceData.data) continue;
        for (const item of sourceData.data) {
            const content = item.content || item.abstract || item.description || item.summary || "";
            if (!content || content.length < 30) continue;

            docs.push({
                source: sourceName,
                title: item.title || item.name || "",
                content: content,
                url: item.url || item.link || item.html_url || "",
            });
        }
    }
    return docs;
}

// ─── Controller ───────────────────────────────────────────────────────────────

exports.unifiedSearchPost = async (req, res) => {
    try {
        const { query, limit } = req.body;

        if (!query || typeof query !== "string" || query.trim().length === 0) {
            return res.status(400).json({
                success: false,
                message: "query must be a non-empty string",
            });
        }

        const maxResults = limit ? parseInt(limit, 10) : 5;
        const startTime  = Date.now();

        console.log(`\n[Unified] ─── New Search ────────────────────`);
        console.log(`[Unified] Query : "${query}" | Limit: ${maxResults}`);

        // ── Phase 0: Route ────────────────────────────────────────────────────
        const routing = await routeQuery(query);

        const domainSourceNames = routing.sources.filter((s) => s !== "duckduckgo");
        const hasBaseline = routing.sources.includes("duckduckgo");

        console.log(`[Unified] Strategy  : ${routing.strategy}`);
        console.log(`[Unified] Categories: ${routing.categories.join(", ") || "none"}`);
        console.log(`[Unified] Sources   : [${domainSourceNames.join(", ")}] + [duckduckgo]`);

        // ── Phase 1: Domain-specific sources (concurrent) ─────────────────────
        const phase1Results = await Promise.all(
            domainSourceNames.map((name) => runSource(name, query, maxResults))
        );

        // ── Phase 2: DuckDuckGo baseline (always last) ────────────────────────
        let ddgResult = null;
        if (hasBaseline) {
            ddgResult = await runSource("duckduckgo", query, maxResults);
        }

        // ── Build sources map ─────────────────────────────────────────────────
        const sources = {};
        let totalResults = 0;

        for (const r of phase1Results) {
            sources[r.name] = r.status === "ok"
                ? { status: "ok", count: r.count, data: r.data }
                : { status: r.status, error: r.error };
            if (r.status === "ok") totalResults += r.count;
        }

        if (ddgResult) {
            sources["duckduckgo"] = ddgResult.status === "ok"
                ? { status: "ok", count: ddgResult.count, data: ddgResult.data }
                : { status: ddgResult.status, error: ddgResult.error };
            if (ddgResult.status === "ok") totalResults += ddgResult.count;
        }

        // ── Phase 3: TF-IDF Research Highlights ───────────────────────────────
        console.log(`[Unified] Phase 3: Extracting research highlights...`);
        const docsForSummarizer = flattenForSummarizer(sources);
        let highlights = [];
        if (docsForSummarizer.length > 0) {
            highlights = await summarizeResults(query, docsForSummarizer, 5);
        }

        const ok     = Object.values(sources).filter((s) => s.status === "ok").length;
        const failed = Object.values(sources).filter((s) => s.status === "failed").length;
        const duration = Date.now() - startTime;

        console.log(`[Unified] Done — ${ok} ok | ${failed} failed | ${totalResults} results | ${highlights.length} highlights | ${duration}ms`);
        console.log(`[Unified] ──────────────────────────────────────\n`);

        res.json({
            success: true,
            query,
            routing: {
                strategy:   routing.strategy,
                categories: routing.categories,
                sources:    routing.sources,
            },
            timing: {
                startTime: new Date(startTime).toISOString(),
                duration:  `${duration}ms`,
            },
            highlights,
            totalResults,
            sourcesQueried:   Object.keys(sources).length,
            sourcesSucceeded: ok,
            sourcesFailed:    failed,
            sources,
        });
    } catch (error) {
        console.error(`[Unified] Unhandled error: ${error.message}`);
        res.status(500).json({ success: false, message: error.message });
    }
};
