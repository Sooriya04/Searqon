/**
 * Searqon Stream Controller (SSE)
 *
 * Flow:
 *  1. Classify query → get ordered source list (Semantic Intent Engine)
 *  2. Execute domain-specific sources concurrently (phase 1)
 *  3. Execute DuckDuckGo last as the baseline (phase 2)
 *  4. Format sources and highlights and send as SSE.
 *  5. Stream the AI Answer token-by-token via SSE.
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
const { synthesizeAnswerStream, extractKnowledgePanel } = require("../services/extractionService");

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
        return { name, status: "failed", error: msg || "Unknown error" };
    }
}

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

exports.streamSearchGet = async (req, res) => {
    try {
        const { query, limit } = req.query;

        if (!query || typeof query !== "string" || query.trim().length === 0) {
            return res.status(400).json({ error: "query must be a non-empty string" });
        }

        const maxResults = limit ? parseInt(limit, 10) : 5;
        const startTime  = Date.now();

        // Standard SSE Headers
        res.setHeader('Content-Type', 'text/event-stream');
        res.setHeader('Cache-Control', 'no-cache');
        res.setHeader('Connection', 'keep-alive');
        // Tell the client that the connection is open
        res.write(`data: ${JSON.stringify({ type: 'status', data: 'Routing Query...' })}\n\n`);

        const routing = await routeQuery(query);
        const domainSourceNames = routing.sources.filter((s) => s !== "duckduckgo");
        const hasBaseline = routing.sources.includes("duckduckgo");

        res.write(`data: ${JSON.stringify({ type: 'status', data: 'Retrieving sources...' })}\n\n`);

        const phase1Results = await Promise.all(
            domainSourceNames.map((name) => runSource(name, query, maxResults))
        );

        let ddgResult = null;
        if (hasBaseline) {
            ddgResult = await runSource("duckduckgo", query, maxResults);
        }

        const sources = {};
        let totalResults = 0;

        for (const r of phase1Results) {
            sources[r.name] = r.status === "ok" ? { status: "ok", count: r.count, data: r.data } : { status: r.status, error: r.error };
            if (r.status === "ok") totalResults += r.count;
        }

        if (ddgResult) {
            sources["duckduckgo"] = ddgResult.status === "ok" ? { status: "ok", count: ddgResult.count, data: ddgResult.data } : { status: ddgResult.status, error: ddgResult.error };
            if (ddgResult.status === "ok") totalResults += ddgResult.count;
        }

        res.write(`data: ${JSON.stringify({ type: 'sources', data: sources })}\n\n`);
        res.write(`data: ${JSON.stringify({ type: 'status', data: 'Extracting highlights...' })}\n\n`);

        const docsForSummarizer = flattenForSummarizer(sources);
        let highlights = [];
        if (docsForSummarizer.length > 0) {
            highlights = await summarizeResults(query, docsForSummarizer, 5);
            res.write(`data: ${JSON.stringify({ type: 'highlights', data: highlights })}\n\n`);
        }

        res.write(`data: ${JSON.stringify({ type: 'status', data: 'Synthesizing answer...' })}\n\n`);

        if (docsForSummarizer.length > 0) {
            // Kick off Knowledge Panel extract asynchronously
            extractKnowledgePanel(query, docsForSummarizer).then(panel => {
                if (panel) {
                    res.write(`data: ${JSON.stringify({ type: 'knowledge_panel', data: panel })}\n\n`);
                }
            }).catch(() => {});

            try {
                const stream = synthesizeAnswerStream(query, docsForSummarizer);
                for await (const chunkStr of stream) {
                    res.write(`data: ${chunkStr}`);
                }
            } catch (err) {
                console.warn(`[Stream] Stream synthesis failed: ${err.message}`);
                res.write(`data: ${JSON.stringify({ type: 'error', data: 'Failed to synthesize text.' })}\n\n`);
            }
        }

        const duration = Date.now() - startTime;
        const durationStr = duration + "ms";
        res.write(`data: ${JSON.stringify({ type: 'done', data: { duration: durationStr } })}\n\n`);
        res.end();
        
    } catch (error) {
        console.error(`[Stream] Unhandled error: ${error.message}`);
        res.write(`data: ${JSON.stringify({ type: 'error', data: error.message })}\n\n`);
        res.end();
    }
};
