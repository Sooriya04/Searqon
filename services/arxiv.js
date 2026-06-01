const xml2js = require('xml2js');
const ScrapUrl = require('../scrapper/ScrapUrl');
const { cleanText } = require('../utils/textCleaner');

const ARXIV_API = 'http://export.arxiv.org/api/query';

async function searchArxiv(query, limit = 10) {
    console.log(`[Arxiv] Searching for: "${query}"`);

    if (!query || typeof query !== 'string') {
        throw new Error('Valid query is required');
    }

    const params = new URLSearchParams({
        search_query: `all:${query}`,
        start: 0,
        max_results: limit,
        sortBy: 'relevance',
        sortOrder: 'descending',
    });

    const res = await fetch(`${ARXIV_API}?${params.toString()}`);
    if (!res.ok) {
        throw new Error(`Arxiv API returned HTTP ${res.status}`);
    }
    const xmlData = await res.text();

    const parsed = await xml2js.parseStringPromise(xmlData, {
        explicitArray: false,
    });

    const entries = parsed.feed?.entry || [];
    const list = Array.isArray(entries) ? entries : [entries];
    // Scrape each arxiv page for content in parallel
    const savedResults = await Promise.all(list.map(async (item, index) => {
        const title = cleanText(item.title);
        const summary = cleanText(item.summary);
        const authors = item.author
            ? Array.isArray(item.author)
                ? item.author.map((a) => a.name)
                : [item.author.name]
            : [];
        const arxivUrl = item.id;

        if (title && summary && summary.length >= 50) {
            let content = summary;
            if (arxivUrl && index < 3) {
                try {
                    const scraped = await ScrapUrl(arxivUrl);
                    if (scraped && scraped.content && scraped.content.length > summary.length) {
                        content = cleanText(scraped.content);
                    }
                } catch (e) { }
            }
            return {
                query: query,
                source: 'arxiv',
                title: title,
                summary: summary,
                content: content,
                authors: authors,
                published: item.published,
                updated: item.updated,
                link: arxivUrl,
                wordCount: content.split(/\s+/).length,
            };
        }
        return null;
    }));

    const filteredResults = savedResults.filter(r => r !== null);
    console.log(`[Arxiv] Returning ${filteredResults.length} results`);
    return filteredResults;
}

module.exports = { searchArxiv };
