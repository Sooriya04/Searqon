const axios = require('axios');
const ScrapUrl = require('../scrapper/ScrapUrl');

const DOAJ_API = 'https://doaj.org/api/v2/search/journals';

function cleanText(text) {
    if (!text || typeof text !== 'string') return '';
    return text
        .replace(/\r\n|\r|\n/g, ' ')
        .replace(/\t/g, ' ')
        .replace(/\s{2,}/g, ' ')
        .trim();
}

async function searchDOAJ(query, limit = 5) {
    console.log(`[DOAJ] Searching for: "${query}"`);

    if (!query || typeof query !== 'string') {
        throw new Error('Valid query is required');
    }

    try {
        const response = await axios.get(`${DOAJ_API}/${encodeURIComponent(query)}`, {
            params: {
                pageSize: limit,
            },
            timeout: 10000,
        });

        const results = response.data?.results || [];
        const savedResults = [];

        for (const item of results) {
            const bib = item.bibjson || {};

            const title = bib.title?.trim();
            const publisherName = typeof bib.publisher === 'object'
                ? bib.publisher?.name || ''
                : bib.publisher || '';
            const subjects = bib.subject
                ? bib.subject.map((s) => s.term).filter(Boolean)
                : [];

            const country = (typeof bib.publisher === 'object'
                ? bib.publisher?.country
                : bib.country) || '';

            const issns = [];
            if (bib.pissn) issns.push(bib.pissn);
            if (bib.eissn) issns.push(bib.eissn);
            if (bib.identifier && Array.isArray(bib.identifier)) {
                bib.identifier
                    .filter((id) => id.type === 'pissn' || id.type === 'eissn')
                    .forEach((id) => { if (id.id && !issns.includes(id.id)) issns.push(id.id); });
            }

            // Try homepage link, then ref link, then construct DOAJ page URL
            const journalUrl =
                bib.ref?.journal ||
                bib.link?.find((l) => l.type === 'homepage')?.url ||
                bib.link?.[0]?.url ||
                (item.id ? `https://doaj.org/toc/${item.id}` : null);

            const summary = `Publisher: ${publisherName}. Country: ${country}. Subjects: ${subjects.join(', ')}`;

            let content = summary;

            // Always scrape content
            if (journalUrl) {
                try {
                    console.log(`[DOAJ] Scraping: ${journalUrl}`);
                    const scraped = await ScrapUrl(journalUrl);

                    if (scraped && scraped.content && scraped.content.length > summary.length) {
                        content = cleanText(scraped.content);
                    }
                } catch (e) {
                    console.log(`[DOAJ] Scraping failed for ${journalUrl}: ${e.message}`);
                }
            }

            if (title) {
                savedResults.push({
                    query: query,
                    source: 'doaj',
                    title: title,
                    summary: summary,
                    content: content,
                    link: journalUrl,
                    wordCount: content.split(/\s+/).length,
                });
            }
        }

        console.log(`[DOAJ] Returning ${savedResults.length} results`);
        return savedResults;

    } catch (error) {
        console.error(`[DOAJ] API Error: ${error.message}`);
        throw new Error('Failed to fetch DOAJ results');
    }
}

module.exports = { searchDOAJ };
