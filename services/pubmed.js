const axios = require('axios');
const cheerio = require('cheerio');
const { cleanText } = require('../utils/textCleaner');
const DatabaseClient = require('../connection/client');

const PUBMED_BASE_URL = 'https://pubmed.ncbi.nlm.nih.gov';

/**
 * Extract PMID from a PubMed URL
 */
function extractPMID(url) {
  const match = url.match(/\/(\d+)\/?$/);
  return match ? match[1] : null;
}

/**
 * Fetch abstract from individual PubMed page
 */
async function fetchAbstract(pmid) {
  const url = `${PUBMED_BASE_URL}/${pmid}/`;

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

    // Extract full abstract
    const abstractText = [];
    $('.abstract-content p').each((i, el) => {
      const label = $(el).find('strong.sub-title').text().trim();
      const text = cleanText($(el).text().replace(label, ''));
      if (text) {
        abstractText.push(label ? `${label}: ${text}` : text);
      }
    });

    return (
      abstractText.join('\n\n') || cleanText($('#eng-abstract').text()) || ''
    );
  } catch (error) {
    console.error(
      `[PubMed] Failed to fetch abstract for PMID ${pmid}: ${error.message}`,
    );
    return '';
  }
}

/**
 * Search PubMed for articles matching the query
 * @param {string} query - Search query
 * @param {number} limit - Maximum number of results to return
 * @returns {Promise<Object>} Search results with title, abstract, url only
 */
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

    // Fetch abstract for each article
    const results = [];
    for (const basic of basicResults) {
      const abstract = await fetchAbstract(basic.pmid);

      const result = {
        title: basic.title,
        abstract,
        url: basic.url,
      };

      results.push(result);

      // Save to database
      await DatabaseClient.saveResult({
        query,
        source: 'pubmed',
        title: result.title,
        url: result.url,
        content: result.abstract,
        wordCount: result.abstract ? result.abstract.split(/\s+/).length : 0,
        score: 0.85,
        metadata: { extraction_method: 'pubmed_scraper' },
      });
    }

    console.log(`[PubMed] Extracted ${results.length} articles`);

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
