const ScrapUrl = require('../scrapper/ScrapUrl');
const WIKI_SEARCH_API = 'https://en.wikipedia.org/w/api.php';

async function searchWikiTitle(query) {
    try {
        const params = new URLSearchParams({
            action: 'opensearch',
            search: query,
            limit: 1,
            format: 'json',
        });
        const response = await fetch(`${WIKI_SEARCH_API}?${params.toString()}`, {
            headers: {
                'User-Agent': 'SearqonBot/1.0',
            },
        });

        if (!response.ok) {
            throw new Error(`Wikipedia returned HTTP ${response.status}`);
        }
        const data = await response.json();

        const titles = data[1];
        return titles && titles.length > 0 ? titles[0] : null;
    } catch (err) {
        console.error(`[Wiki] Search failed: ${err.message}`);
        return null;
    }
}

async function wikiSearch(query) {
    console.log(`[Wiki] Searching for: "${query}"`);

    // First, find the correct article title
    const title = await searchWikiTitle(query);

    if (!title) {
        throw new Error(`No Wikipedia article found for "${query}"`);
    }

    console.log(`[Wiki] Found article: "${title}"`);

    // Extract content from the article using Scrapper (Always scrape)
    const url = `https://en.wikipedia.org/wiki/${title.replace(/ /g, '_')}`;
    let extracted = { title, url, content: '', wordCount: 0 };

    console.log(`[Wiki] Scraping: ${url}`);
    extracted = await ScrapUrl(url);
    console.log(`[Wiki] Scraped ${extracted.wordCount} words from "${title}"`);

    // Save to database microservice
    const resultData = {
        query: query,
        source: 'wikipedia',
        title: extracted.title || title,
        url: extracted.url || url,
        content: extracted.content || 'Content extraction failed',
        wordCount: extracted.wordCount || 0,
        score: 0.9,
        metadata: {
            article_title: title,
            extraction_method: 'worker_pool_scraper',
        },
    };
    return {
        query: query,
        source: 'wikipedia',
        title: resultData.title,
        url: resultData.url,
        content: resultData.content,
        wordCount: resultData.wordCount,
        score: resultData.score,
    };
}

module.exports = {
    wikiSearch,
};
