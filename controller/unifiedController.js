/**
 * Searqon Unified Search Controller (Ultra-Minimal Version)
 *
 * Optimized for speed and direct JSON consumption.
 * Returns a flat array of results with extracted content.
 */

const { searchArxiv }        = require("../services/arxiv");
const { searchDOAJ }         = require("../services/doaj");
const { searchMedRxiv }      = require("../services/medrxiv");
const { searchTalven }       = require("../provider/talven");
const { searchDuckDuckGo }   = require("../services/duckduckgo");
const { searchOpenAlex }     = require("../services/openalex");
const { searchPubMed }       = require("../services/pubmed");
const { searchHNByQuery }    = require("../services/hackernews");
const { wikiSearch } = require("../services/wiki");
const { searchWithReadmes }  = require("../services/github");
const { reddit } = require("../services/reddit");
const { searchWeb } = require("../services/web");
const { searchGeeksForGeeks} = require("../services/geeksforgeeks");
const { searchYoutube } = require("../services/youtube");
const { routeQuery } = require("../services/classifierService");
const config = require("../utils/configLoader");
const { rerankResults } = require("../services/rerankService");

const ALL_SOURCES = [
    { name: "arxiv",         fn: (q, l)         => searchArxiv(q, l)                                },
    { name: "doaj",          fn: (q, l)         => searchDOAJ(q, l)                                 },
    { name: "medrxiv",       fn: (q, l)         => searchMedRxiv(q, l)                              },
    { name: "openalex",      fn: (q, l)         => searchOpenAlex(q, l)                             },
    { name: "pubmed",        fn: (q, l)         => searchPubMed(q, l)                               },
    { name: "hackernews",    fn: (q, l)         => searchHNByQuery(q, l)                            },
    { name: "wikipedia",     fn: (q)            => wikiSearch(q)                                    },
    { name: "github",        fn: (q, l)         => searchWithReadmes(q, l)                          },
    { name: "reddit",        fn: (q, l)         => reddit(q, l)                                     },
    { name: "geeksforgeeks", fn: (q, l)         => searchGeeksForGeeks(q, l)                        },
    { name: "youtube",       fn: (q, l)         => searchYoutube(q, l)                              },
    { name: "web",           fn: (q, l, options) => searchWeb(q, l, options)                          },
    { name: "duckduckgo",    fn: (q, l, options) => searchDuckDuckGo(q, l, options)                   },
    { name: "talven",        fn: (q, l, options) => searchTalven(q, l, options)                       },
];

const SOURCE_MAP = Object.fromEntries(ALL_SOURCES.map((s) => [s.name, s]));

async function runSource(name, query, limit, options = {}) {
    const def = SOURCE_MAP[name];
    if (!def) return [];
    try {
        const raw = await Promise.race([
            def.fn(query, limit, options),
            new Promise((_, reject) => setTimeout(() => reject(new Error("Timeout")), 1800))
        ]);
        const data = (raw.results || (Array.isArray(raw) ? raw : [raw])).filter(Boolean);
        return data.map(item => ({
            title:   item.title || item.name || "No Title",
            url:     item.url || item.link || item.html_url || "",
            content: item.content || item.abstract || item.description || item.summary || "",
            source:  name
        }));
    } catch (err) {
        return [];
    }
}

exports.unifiedSearchPost = async (req, res) => {
    try {
        const { query, limit } = req.body;
        if (!query) return res.status(400).json({ error: "query required" });

        const maxResults = limit ? parseInt(limit, 10) : 5;
        const startTime  = Date.now();

        // 0. Check PostgreSQL Cache
        const { getCachedSearchResults, saveSearchResult } = require("../models/result");
        try {
            const cached = await getCachedSearchResults(query);
            if (cached && cached.length > 0) {
                console.log(`[Unified] Cache HIT: returning ${cached.length} cached results for: "${query}"`);
                const duration = "0.00";
                res.set('X-Search-Duration', `${duration}s`);
                return res.json(cached.slice(0, maxResults).map(r => ({
                    title: r.title,
                    url: r.url,
                    content: r.content,
                    markdown: r.metadata?.markdown || null,
                    source: r.source,
                    metadata: {
                        snippet: r.metadata?.snippet || r.content,
                        extraction_method: 'postgres_cache'
                    }
                })));
            }
        } catch (err) {
            console.warn(`[Unified] Cache check failed, proceeding to live search: ${err.message}`);
        }

        // 1. Route Query
        const routing = await routeQuery(query);
        const domainSources = routing.sources.filter(s => s !== "duckduckgo" && s !== "talven");
        
        // 2. Parallel Search Discovery (Skip scraping initially to minimize latency/bandwidth)
        const options = { skipScrape: true };
        const searchPromises = domainSources.map(name => runSource(name, query, maxResults, options));
        
        const useTalven = config.providers?.talven !== false;
        
        // Always include DuckDuckGo as the normal web search baseline if general search is needed
        if (routing.sources.includes("duckduckgo") || routing.sources.includes("talven") || routing.sources.length === 0) {
            searchPromises.push(runSource("duckduckgo", query, maxResults, options));
        }
        
        // If Talven is enabled, it ALWAYS adds 3 additional results on top of the limit
        if (useTalven) {
            searchPromises.push(runSource("talven", query, 3, options));
        }

        const taskResults = await Promise.all(searchPromises);
        let flatResults = taskResults.flat();

        // 3. Intelligent Reranking
        if (flatResults.length > 0) {
            flatResults = await rerankResults(query, flatResults, maxResults);
        }

        // 4. Parallel Page Scraping Phase (Only scrape the final top reranked results!)
        const urlsToScrape = flatResults.map(r => r.url).filter(Boolean);
        let scrapedDataMap = {};
        if (urlsToScrape.length > 0) {
            const { ScrapUrlBatch } = require('../scrapper/ScrapUrl');
            console.log(`[Unified] Scraping final ${urlsToScrape.length} search results concurrently using Go Batch Scraper...`);
            try {
                const batchResults = await ScrapUrlBatch(urlsToScrape, { format: 'markdown' });
                batchResults.forEach(scraped => {
                    if (scraped && scraped.url) {
                        scrapedDataMap[scraped.url] = scraped;
                    }
                });
            } catch (err) {
                console.warn(`[Unified] Batch scraping search results failed:`, err.message);
            }
        }

        // Map scraped content back to flatResults
        const richResults = flatResults.map(r => {
            const pageData = scrapedDataMap[r.url];
            return {
                title:    (pageData && pageData.title) || r.title,
                url:      r.url,
                content:  (pageData && pageData.content) || r.content,
                markdown: (pageData && pageData.markdown) || null,
                source:   r.source,
                metadata: {
                    snippet: r.content,
                    extraction_method: pageData ? 'parallel_go_batch' : 'snippet_only'
                }
            };
        });

        // 5. Save results to PostgreSQL Cache asynchronously
        richResults.forEach(r => {
            saveSearchResult({
                query,
                source: r.source,
                title: r.title,
                url: r.url,
                content: r.content,
                score: 0.8,
                wordCount: r.content ? r.content.split(/\s+/).length : 0,
                metadata: {
                    snippet: r.metadata?.snippet,
                    markdown: r.markdown,
                    extraction_method: r.metadata?.extraction_method
                }
            }).catch(err => console.error('[Unified] Failed to cache result in Postgres:', err.message));
        });

        const duration = ((Date.now() - startTime) / 1000).toFixed(2);

        // 6. Return Flat JSON Array
        res.set('X-Search-Duration', `${duration}s`);
        res.json(richResults);

    } catch (error) {
        res.status(500).json({ error: error.message });
    }
};
