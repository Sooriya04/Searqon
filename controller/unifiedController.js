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
    { name: "talven",        fn: (q, l) => searchTalven(q, l)        },
];

const SOURCE_MAP = Object.fromEntries(ALL_SOURCES.map((s) => [s.name, s]));

async function runSource(name, query, limit) {
    const def = SOURCE_MAP[name];
    if (!def) return [];
    try {
        const raw = await Promise.race([
            def.fn(query, limit),
            new Promise((_, reject) => setTimeout(() => reject(new Error("Timeout")), 8000))
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

        // 1. Route Query
        const routing = await routeQuery(query);
        const domainSources = routing.sources.filter(s => s !== "duckduckgo" && s !== "talven");
        
        // 2. Parallel Search
        const searchPromises = domainSources.map(name => runSource(name, query, maxResults));
        
        const useTalven = config.providers?.talven !== false;
        
        // Always include DuckDuckGo as the normal web search baseline if general search is needed
        if (routing.sources.includes("duckduckgo") || routing.sources.includes("talven") || routing.sources.length === 0) {
            searchPromises.push(runSource("duckduckgo", query, maxResults));
        }
        
        // If Talven is enabled, it ALWAYS adds 3 additional results on top of the limit
        if (useTalven) {
            searchPromises.push(runSource("talven", query, 3));
        }

        const taskResults = await Promise.all(searchPromises);
        const flatResults = taskResults.flat();

        const duration = ((Date.now() - startTime) / 1000).toFixed(2);

        // 3. Return Flat JSON Array as requested
        // Added a small "meta" header if needed, or just the array. 
        // User said "send the json alone", so I'll send the array directly.
        res.set('X-Search-Duration', `${duration}s`);
        res.json(flatResults);

    } catch (error) {
        res.status(500).json({ error: error.message });
    }
};
