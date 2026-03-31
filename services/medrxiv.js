const axios = require('axios');
const ScrapUrl = require('../scrapper/ScrapUrl');

// Europe PMC indexes medRxiv preprints and provides a proper search API
const EUROPEPMC_API = 'https://www.ebi.ac.uk/europepmc/webservices/rest/search';

function cleanText(text) {
    if (!text || typeof text !== 'string') return '';
    return text
        .replace(/\r\n|\r|\n/g, ' ')
        .replace(/\t/g, ' ')
        .replace(/<[^>]*>/g, '')
        .replace(/\s{2,}/g, ' ')
        .trim();
}

async function searchMedRxiv(query, limit = 5) {
    console.log(`[MedRxiv] Searching for: "${query}"`);

    if (!query || typeof query !== 'string') {
        throw new Error('Valid query is required');
    }

    try {
        const response = await axios.get(EUROPEPMC_API, {
            params: {
                query: `(SRC:PPR) AND (PUBLISHER:"medRxiv") AND (${query})`,
                resultType: 'core',
                pageSize: limit,
                format: 'json',
            },
            timeout: 45000,
        });

        const results = response.data?.resultList?.result || [];
        const savedResults = await Promise.all(results.map(async (item, index) => {
            const title = cleanText(item.title || '');
            const summary = cleanText(item.abstractText || '');
            const articleUrl = item.doi
                ? `https://doi.org/${item.doi}`
                : item.fullTextUrlList?.fullTextUrl?.[0]?.url || null;

            let content = summary;
            if (articleUrl && index < 3) {
                try {
                    const scraped = await ScrapUrl(articleUrl);
                    if (scraped && scraped.content && scraped.content.length > summary.length) {
                        content = cleanText(scraped.content);
                    }
                } catch (e) { }
            }

            if (title) {
                return {
                    query: query,
                    source: 'medrxiv',
                    title: title,
                    summary: summary,
                    content: content,
                    link: articleUrl,
                    wordCount: content.split(/\s+/).length,
                };
            }
            return null;
        }));

        const filteredResults = savedResults.filter(r => r !== null);
        console.log(`[MedRxiv] Returning ${filteredResults.length} results`);
        return filteredResults;

    } catch (error) {
        console.error(`[MedRxiv] API Error: ${error.message}`);
        throw new Error('Failed to fetch MedRxiv results');
    }
}

module.exports = { searchMedRxiv };
