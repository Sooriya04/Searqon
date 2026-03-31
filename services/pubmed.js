const axios = require('axios');
const cheerio = require('cheerio');
const ScrapUrl = require('../scrapper/ScrapUrl');
const { cleanText } = require('../utils/textCleaner');

const PUBMED_BASE_URL = 'https://pubmed.ncbi.nlm.nih.gov';

function extractPMID(url) {
    const match = url.match(/\/(\d+)\/?$/);
    return match ? match[1] : null;
}
async function searchPubMed(query, limit = 10) {
    if (!query) {
        throw new Error('Query is required');
    }

    console.log(`[PubMed] Searching for: "${query}"`);

    const url = `${PUBMED_BASE_URL}/?term=${encodeURIComponent(query)}`;

    try {
        const response = await axios.get(url, {
            headers: {
                'User-Agent':
                    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36',
                Accept: 'text/html',
            },
            timeout: 15000,
        });

        const $ = cheerio.load(response.data);
        const basicResults = [];

        // Get basic info from search results
        $('.docsum-content').each((i, el) => {
            if (i >= limit) return false;

            const title = cleanText($(el).find('.docsum-title').text());
            const relativeLink = $(el).find('.docsum-title').attr('href');
            const link = relativeLink ? `${PUBMED_BASE_URL}${relativeLink}` : null;
            const pmid = extractPMID(link);

            if (title && link && pmid) {
                basicResults.push({ pmid, title, url: link });
            }
        });

        console.log(
            `[PubMed] Found ${basicResults.length} results, fetching abstracts...`,
        );
        const results = await Promise.all(basicResults.map(async (basic, index) => {
            let abstract = '';
            try {
                if (index < 3) {
                    const scrapeResult = await ScrapUrl(basic.url);
                    abstract = scrapeResult.content || '';
                }
            } catch (e) {
                console.error(`[PubMed] Scraping failed for ${basic.url}: ${e.message}`);
            }
            return {
                title: basic.title,
                abstract,
                url: basic.url,
            };
        }));
        return {
            query,
            source: 'pubmed',
            count: results.length,
            results,
        };
    } catch (error) {
        console.error(`[PubMed] Search failed: ${error.message}`);
        throw new Error(`Failed to fetch PubMed results: ${error.message}`);
    }
}

module.exports = {
    searchPubMed,
};
