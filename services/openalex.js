const axios = require('axios');
const ScrapUrl = require('../scrapper/ScrapUrl');

const OPENALEX_API = 'https://api.openalex.org/works';

/**
 * Search OpenAlex for paper URLs, then scrape each for content.
 * OpenAlex is free, no API key, no rate limiting.
 */
async function searchOpenAlex(query, limit = 10) {
    if (!query) {
        throw new Error('Query is required');
    }

    console.log(`[OpenAlex] Searching: "${query}"`);

    try {
        const response = await axios.get(OPENALEX_API, {
            params: {
                search: query,
                per_page: limit,
                select: 'display_name,primary_location,doi,id,locations,best_oa_location',
            },
            headers: {
                Accept: 'application/json',
                'User-Agent': 'Searqon/1.0 (mailto:searqon@example.com)',
            },
            timeout: 30000,
        });

        const papers = response.data?.results || [];
        console.log(`[OpenAlex] API returned ${papers.length} papers`);

        const basicResults = [];

        for (const p of papers) {
            const title = p.display_name || '';

            // Try multiple URL sources — prefer open access URLs (less likely to 403)
            const oaLandingPage = p.best_oa_location?.landing_page_url || p.best_oa_location?.pdf_url;
            const primaryLandingPage = p.primary_location?.landing_page_url;
            const anyLocationUrl = p.locations?.find(l => l.landing_page_url)?.landing_page_url || null;
            const doi = p.doi || null;

            const url = oaLandingPage || primaryLandingPage || anyLocationUrl || doi || null;

            if (title && url) {
                basicResults.push({ title, url });
            }
        }

        console.log(
            `[OpenAlex] ${basicResults.length}/${papers.length} papers have usable URLs, scraping content...`,
        );

        // Scrape each paper URL for content in parallel
        const results = await Promise.all(basicResults.map(async (basic) => {
            let content = '';
            try {
                const scrapeResult = await ScrapUrl(basic.url);
                content = scrapeResult.content || '';
            } catch (e) {
                console.error(`[OpenAlex] Scraping failed for ${basic.url}: ${e.message}`);
            }
            return {
                title: basic.title,
                content,
                url: basic.url,
            };
        }));

        return {
            query,
            source: 'openalex',
            count: results.length,
            results,
        };
    } catch (error) {
        console.error(`[OpenAlex] Search failed: ${error.message}`);
        throw new Error(
            `Failed to fetch OpenAlex results: ${error.message}`,
        );
    }
}

module.exports = {
    searchOpenAlex,
};
