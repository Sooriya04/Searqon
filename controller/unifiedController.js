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

// All sources with their search functions and labels
const SOURCES = [
    { name: "web", fn: (q, l) => searchWeb(q, l) },
    { name: "arxiv", fn: (q, l) => searchArxiv(q, l) },
    { name: "doaj", fn: (q, l) => searchDOAJ(q, l) },
    { name: "medrxiv", fn: (q, l) => searchMedRxiv(q, l) },
    { name: "duckduckgo", fn: (q, l) => searchDuckDuckGo(q, l) },
    { name: "openalex", fn: (q, l) => searchOpenAlex(q, l) },
    { name: "pubmed", fn: (q, l) => searchPubMed(q, l) },
    { name: "hackernews", fn: (q, l) => searchHNByQuery(q, l) },
    { name: "wikipedia", fn: (q) => wikiSearch(q) },
    { name: "github", fn: (q, l) => searchWithReadmes(q, l) },
    { name: "reddit", fn: (q, l) => reddit(q, l) },
];


function normalizeResults(data) {
    if (!data) return [];

    // PubMed / OpenAlex shape: { query, source, count, results }
    if (data.results && Array.isArray(data.results)) {
        return data.results;
    }

    // Already an array (arxiv, doaj, medrxiv, duckduckgo, hackernews, github, reddit)
    if (Array.isArray(data)) {
        return data;
    }

    // Single object (wiki)
    return [data];
}

exports.unifiedSearchPost = async (req, res) => {
    try {
        const { query, limit } = req.body;

        if (!query || typeof query !== "string" || query.trim().length === 0) {
            return res.status(400).json({
                success: false,
                message: "Query must be a non-empty string",
            });
        }

        const maxResults = limit ? parseInt(limit, 10) : 5;

        const startTime = Date.now();
        console.log(`[Unified] Search: "${query}" | Limit: ${maxResults}`);

        // Fire all searches concurrently
        const promises = SOURCES.map((source) => ({
            name: source.name,
            promise: source.fn(query, maxResults),
        }));

        const settled = await Promise.allSettled(
            promises.map((p) => p.promise)
        );

        // Build the sources response object
        const sources = {};
        let totalResults = 0;

        settled.forEach((result, index) => {
            const sourceName = promises[index].name;

            if (result.status === "fulfilled") {
                const data = normalizeResults(result.value);
                sources[sourceName] = {
                    status: "ok",
                    count: data.length,
                    data: data,
                };
                totalResults += data.length;
            } else {
                console.error(`[Unified] ${sourceName} failed: ${result.reason?.message}`);
                sources[sourceName] = {
                    status: "failed",
                    error: result.reason?.message || "Unknown error",
                };
            }
        });

        const successCount = Object.values(sources).filter((s) => s.status === "ok").length;
        const failedCount = Object.values(sources).filter((s) => s.status === "failed").length;

        const endTime = Date.now();
        const duration = endTime - startTime;

        console.log(`[Unified] Done — ${successCount} sources ok, ${failedCount} failed, ${totalResults} total results (Duration: ${duration}ms)`);

        res.json({
            success: true,
            timing: {
                startTime: new Date(startTime).toISOString(),
                endTime: new Date(endTime).toISOString(),
                duration: `${duration}ms`
            },
            query,
            totalResults,
            sourcesQueried: SOURCES.length,
            sourcesSucceeded: successCount,
            sourcesFailed: failedCount,
            sources,
        });
    } catch (error) {
        res.status(500).json({
            success: false,
            message: error.message,
        });
    }
};
