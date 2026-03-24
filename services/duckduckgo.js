const cheerio = require('cheerio');
const httpClient = require('../utils/httpClient');
const { BROWSER_HEADERS } = require('../utils/browserHeaders');
const { cleanText, cleanSearchSnippet } = require('../utils/textCleaner');
const ScrapUrl = require('../scrapper/ScrapUrl');

const DUCKDUCKGO_URL = 'https://html.duckduckgo.com/html/';

function isAdUrl(url) {
  return url && url.includes('duckduckgo.com/y.js');
}

async function fetchSearchResults(query) {
  const response = await httpClient.post(
    DUCKDUCKGO_URL,
    new URLSearchParams({ q: query }).toString(),
    {
      headers: {
        ...BROWSER_HEADERS,
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      responseType: 'text',
    },
  );

  if (!response || typeof response.data !== 'string') {
    throw new Error('DuckDuckGo returned no HTML');
  }

  return parseSearchResults(response.data);
}

function parseSearchResults(html) {
  const $ = cheerio.load(html);
  const results = [];

  $('.result').each((i, el) => {
    const $el = $(el);
    const link = $el.find('.result__a');
    const title = cleanText(link.text());
    const url = link.attr('href');
    const snippet = cleanText($el.find('.result__snippet').text());

    if (title && url && !isAdUrl(url)) {
      results.push({ title, url, snippet });
    }
  });

  return results;
}

async function searchDuckDuckGo(query, limit = 5) {
  console.log(`[DuckDuckGo] Searching for: "${query}"`);

  const rawResults = await fetchSearchResults(query);
  console.log(`[DuckDuckGo] Scraping ${rawResults.slice(0, limit).length} results in parallel...`);
  const savedResults = await Promise.all(rawResults.slice(0, limit).map(async (result) => {
    let resultData = null;

    if (result.url) {
      try {
        const pageData = await ScrapUrl(result.url);
        if (pageData && pageData.content && pageData.wordCount >= 10) {
          resultData = {
            query: query,
            source: 'duckduckgo',
            title: pageData.title || result.title,
            url: result.url,
            content: pageData.content,
            score: 0.7,
            wordCount: pageData.wordCount,
            metadata: {
              snippet: result.snippet || '',
              extraction_method: 'worker_pool_scraper',
            },
          };
        }
      } catch (e) {
        // Silently fail scraping
      }
    }

    if (!resultData && result.snippet && result.snippet.length >= 30) {
      const cleanedSnippet = cleanSearchSnippet(result.snippet);
      resultData = {
        query: query,
        source: 'duckduckgo',
        title: result.title,
        url: result.url,
        content: cleanedSnippet,
        score: 0.5,
        wordCount: cleanedSnippet.split(/\s+/).length,
        metadata: {
          snippet: result.snippet,
          extraction_method: 'snippet_only',
        },
      };
    }
    return resultData;
  }));

  const filteredResults = savedResults.filter(r => r !== null);
  console.log(`[DuckDuckGo] Returning ${filteredResults.length} results`);
  return filteredResults;
}

module.exports = { searchDuckDuckGo };
