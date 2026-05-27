
const { searchDuckDuckGo } = require("../services/duckduckgo");
const { searchTalven }     = require("../provider/talven");
const { rerankResults }    = require("../services/rerankService");
const config               = require("../utils/configLoader");

async function searchController(req, res) {
  const { query, maxResults = 5, includeRawContent = true } = req.body;

  // Validate input
  if (!query || typeof query !== "string" || query.trim().length === 0) {
    return res.status(400).json({
      error: "Invalid request body",
      message: "`query` must be a non-empty string",
    });
  }

  try {
    const startTime = Date.now();
    const useTalven = config.providers?.talven !== false;
    const useRerank = config.providers?.rerank !== false;

    // 1. Parallel Discovery (DuckDuckGo + Talven) - Skip page scraping to minimize discovery latency
    const [ddgRes, talvenRes] = await Promise.all([
      searchDuckDuckGo(query.trim(), maxResults, { skipScrape: true }).catch(() => []),
      useTalven ? searchTalven(query.trim(), 3, { skipScrape: true }).catch(() => []) : Promise.resolve([])
    ]);

    // 2. Flatten and map results
    let results = [...ddgRes, ...talvenRes.map(r => ({ ...r, source: 'talven' }))];
    
    // Deduplicate by URL
    const seen = new Set();
    results = results.filter(r => {
      if (!r.url || seen.has(r.url)) return false;
      seen.add(r.url);
      return true;
    });

    // 3. Intelligent Reranking
    // The service internally handles node_rerank vs python_rerank toggles
    if (results.length > 0) {
      results = await rerankResults(query.trim(), results, maxResults);
    }

    // 4. Parallel Page Scraping Phase (Only scrape the final top reranked results!)
    const urlsToScrape = results.map(r => r.url).filter(Boolean);
    let scrapedDataMap = {};
    if (urlsToScrape.length > 0) {
      const { ScrapUrlBatch } = require('../scrapper/ScrapUrl');
      console.log(`[SearchController] Scraping final ${urlsToScrape.length} search results concurrently using Go Batch Scraper...`);
      try {
        const batchResults = await ScrapUrlBatch(urlsToScrape, { format: 'markdown' });
        batchResults.forEach(scraped => {
          if (scraped && scraped.url) {
            scrapedDataMap[scraped.url] = scraped;
          }
        });
      } catch (err) {
        console.warn(`[SearchController] Batch scraping search results failed:`, err.message);
      }
    }

    const responseTime = Date.now() - startTime;
    
    return res.status(200).json({
      success: true,
      query:   query.trim(),
      results: results.map((r) => {
        const pageData = scrapedDataMap[r.url];
        return {
          title:       (pageData && pageData.title) || r.title,
          url:         r.url,
          content:     (pageData && pageData.content) || r.content,
          markdown:    (pageData && pageData.markdown) || null,
          explanation: r.explanation, // New field from reranker
          score:       r.score,
          metadata: {
            source:            r.source || 'duckduckgo',
            duration:          `${responseTime}ms`,
            extraction_method: pageData ? 'parallel_go_batch' : 'snippet_only'
          }
        };
      }),
    });
  } catch (err) {
    console.error("Search execution failed:", err.message);

    return res.status(500).json({
      error: "Search failed",
      reason: err.message,
    });
  }
}

module.exports = { searchController };
