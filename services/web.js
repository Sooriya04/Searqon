const ScrapUrl = require('../scrapper/ScrapUrl');
const config = require('../utils/configLoader');

async function searchWeb(query, limit = 1, options = {}) {
    // Basic URL validation
    try {
        new URL(query);
    } catch (_) {
        // Not a URL, fallback to general search (Talven + DuckDuckGo)
        const { searchDuckDuckGo } = require('./duckduckgo');
        const { searchTalven } = require('../provider/talven');
        
        const useTalven = config.providers?.talven !== false;
        
        const [talvenRes, ddgRes] = await Promise.all([
            useTalven ? searchTalven(query, 3, options).catch(() => []) : Promise.resolve([]),
            searchDuckDuckGo(query, limit, options).catch(() => [])
        ]);

        const seen = new Set();
        const results = [...ddgRes, ...talvenRes].filter(r => {
            if (!r || !r.url || seen.has(r.url)) return false;
            seen.add(r.url);
            return true;
        });

        return results.map(r => ({ ...r, source: 'web' }));
    }

    console.log(`[Web] Scraping URL: ${query}`);

    try {
        const result = await ScrapUrl(query);

        if (!result || !result.content) {
            return [];
        }

        return [{
            query: query,
            source: 'web',
            title: result.title || query,
            url: query,
            content: result.content,
            score: 1.0,
            wordCount: result.wordCount,
            author: 'unknown',
            publishedDate: new Date().toISOString(),
        }];
    } catch (error) {
        console.error(`[Web] Failed to scrape ${query}: ${error.message}`);
        return [];
    }
}

module.exports = {
    searchWeb
};
