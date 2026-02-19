const ScrapUrl = require('../scrapper/ScrapUrl');

async function searchWeb(query, limit = 1) {
    // Basic URL validation
    try {
        new URL(query);
    } catch (_) {
        // Not a URL, return empty (or we could search Google/DDG, but this service is for direct scraping)
        return [];
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
