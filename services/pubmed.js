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
        const response = await fetch(url, {
            headers: {
                'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36',
                Accept: 'text/html',
            },
            signal: AbortSignal.timeout(15000)
        });

        if (!response.ok) {
            throw new Error(`PubMed returned HTTP ${response.status}`);
        }

        const html = await response.text();
        const basicResults = [];

        // Clean HTML to simplify regex parsing
        const cleanHtml = html.replace(/\r?\n|\r/g, ' ');

        // Find each docsum-content block
        const matches = cleanHtml.matchAll(/<div[^>]*class="[^"]*docsum-content[^"]*"[^>]*>([\s\S]*?)<\/div>/g);
        let count = 0;
        for (const match of matches) {
            if (count >= limit) break;
            const block = match[1];

            const linkMatch = block.match(/<a[^>]*class="[^"]*docsum-title[^"]*"[^>]*href="([^"]+)"[^>]*>([\s\S]*?)<\/a>/);
            if (linkMatch) {
                const relativeLink = linkMatch[1];
                const link = `${PUBMED_BASE_URL}${relativeLink}`;
                const title = cleanText(linkMatch[2].replace(/<[^>]+>/g, '').trim());
                const pmid = extractPMID(link);

                if (title && link && pmid) {
                    basicResults.push({ pmid, title, url: link });
                    count++;
                }
            }
        }

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
