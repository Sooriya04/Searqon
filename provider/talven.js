const { cleanSearchSnippet } = require('../utils/textCleaner');
const { ScrapUrlBatch } = require('../scrapper/ScrapUrl');

const TALVEN_URL = process.env.TALVEN_URL || 'http://localhost:8888';

async function fetchTalvenResults(query, categories = '') {
    const params = new URLSearchParams({
        q: query,
        format: 'json',
        language: 'en-US'
    });
    
    if (categories) {
        params.append('categories', categories);
    }
    
    const controller = new AbortController();
    const id = setTimeout(() => controller.abort(), 10000);
    
    try {
        const res = await fetch(`${TALVEN_URL}/search?${params.toString()}`, {
            signal: controller.signal,
            headers: { 'Accept': 'application/json' }
        });
        
        if (!res.ok) {
            throw new Error(`Talven returned HTTP ${res.status}`);
        }
        
        const data = await res.json();
        return data.results || [];
    } finally {
        clearTimeout(id);
    }
}

async function searchTalven(query, limit = 5, categories = '', options = {}) {
    // If the 3rd argument is an object (i.e. options was passed as the 3rd arg), swap them
    let opt = options;
    let cats = categories;
    if (typeof categories === 'object') {
        opt = categories;
        cats = '';
    }

    console.log(`[TalvenProvider] Searching for: "${query}" (limit: ${limit})`);

    let rawResults = [];
    try {
        rawResults = await fetchTalvenResults(query, cats);
    } catch (e) {
        console.warn(`[TalvenProvider] Search failed: ${e.message}. Talven must be running on ${TALVEN_URL}`);
        return [];
    }

    const sliced = rawResults.slice(0, limit);

    if (opt.skipScrape) {
        console.log(`[TalvenProvider] Skipping page scraping, returning raw metadata/snippets only.`);
        return sliced.map(result => ({
            query: query,
            source: 'talven',
            title: result.title,
            url: result.url,
            content: cleanSearchSnippet(result.content),
            markdown: null,
            score: result.score || 0.8,
            wordCount: result.content ? result.content.split(/\s+/).length : 0,
            metadata: {
                snippet: result.content || '',
                extraction_method: 'snippet_only',
                engine: result.engine || 'multiple'
            }
        }));
    }

    console.log(`[TalvenProvider] Scraping top ${sliced.length} results in parallel using Go Batch Api...`);
    
    // Extract URLs to batch
    const urlsToScrape = sliced.map(r => r.url).filter(Boolean);
    
    // Hit the Go Batch Endpoint
    let batchResults = [];
    try {
        if (urlsToScrape.length > 0) {
            // Pass format: 'markdown' to get high-fidelity content
            batchResults = await ScrapUrlBatch(urlsToScrape, { format: 'markdown' });
        }
    } catch (e) {
        console.warn(`[TalvenProvider] Batch scraping failed, falling back to snippets: ${e.message}`);
    }

    // Create a map of URL -> Scraped Content
    const scrapedDataMap = {};
    batchResults.forEach(res => {
        if (res && res.content && res.wordCount >= 10 && !res.error) {
            scrapedDataMap[res.url] = res;
        }
    });

    const savedResults = sliced.map((result) => {
        let resultData = null;

        // Check if we got good scraped data from the batch
        if (result.url && scrapedDataMap[result.url]) {
            const pageData = scrapedDataMap[result.url];
            resultData = {
                query: query,
                source: 'talven',
                title: pageData.title || result.title,
                url: result.url,
                content: pageData.content,
                markdown: pageData.markdown || null,
                score: result.score || 0.8,
                wordCount: pageData.wordCount,
                metadata: {
                    snippet: result.content || '', // Talven uses 'content' for snippet
                    extraction_method: 'go_batch_scraper',
                    engine: result.engine || 'multiple'
                },
            };
        }

        // Fall back to snippet if scraping failed or was too short
        if (!resultData && result.content && result.content.length >= 20) {
            const cleanedSnippet = cleanSearchSnippet(result.content);
            resultData = {
                query: query,
                source: 'talven',
                title: result.title,
                url: result.url,
                content: cleanedSnippet,
                markdown: null,
                score: (result.score || 0.5) * 0.8, // penalty for snippet only
                wordCount: cleanedSnippet.split(/\s+/).length,
                metadata: {
                    snippet: result.content,
                    extraction_method: 'snippet_only',
                    engine: result.engine || 'multiple'
                },
            };
        }
        return resultData;
    });

    const filteredResults = savedResults.filter(r => r !== null);
    console.log(`[TalvenProvider] Returning ${filteredResults.length} structured results`);
    return filteredResults;
}

module.exports = { searchTalven };
