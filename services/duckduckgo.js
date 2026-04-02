const cheerio = require('cheerio');
const httpClient = require('../utils/httpClient');
const { BROWSER_HEADERS } = require('../utils/browserHeaders');
const { cleanText, cleanSearchSnippet } = require('../utils/textCleaner');
const { ScrapUrlBatch } = require('../scrapper/ScrapUrl');

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
  const sliced = rawResults.slice(0, limit);
  console.log(`[DuckDuckGo] Scraping top ${sliced.length} results in parallel using Go Batch Api...`);
  
  // Extract URLs to batch
  const urlsToScrape = sliced.map(r => r.url).filter(Boolean);
  
  // Hit the Go Batch Endpoint
  let batchResults = [];
  try {
      if (urlsToScrape.length > 0) {
          // Pass format: 'markdown' to get high-fidelity content
          batchResults = await ScrapUrlBatch(urlsToScrape, { format: 'markdown' });
      }
  } catch (e) {
      console.warn(`[DuckDuckGo] Batch scraping failed, falling back to snippets: ${e.message}`);
  }

  // Create a map of URL -> Scraped Content
  const scrapedDataMap = {};
  batchResults.forEach(res => {
      if (res && res.content && res.wordCount >= 10 && !res.error) {
          scrapedDataMap[res.url] = res;
      }
  });

  const savedResults = sliced.map((result) => {
    let resultData = null;

    // Check if we got good scraped data from the batch
    if (result.url && scrapedDataMap[result.url]) {
      const pageData = scrapedDataMap[result.url];
      resultData = {
        query: query,
        source: 'duckduckgo',
        title: pageData.title || result.title,
        url: result.url,
        content: pageData.content,
        markdown: pageData.markdown || null,
        score: 0.7,
        wordCount: pageData.wordCount,
        metadata: {
          snippet: result.snippet || '',
          extraction_method: 'go_batch_scraper',
        },
      };
    }

    // Fall back to snippet if scraping failed or was too short
    if (!resultData && result.snippet && result.snippet.length >= 20) {
      const cleanedSnippet = cleanSearchSnippet(result.snippet);
      resultData = {
        query: query,
        source: 'duckduckgo',
        title: result.title,
        url: result.url,
        content: cleanedSnippet,
        markdown: null,
        score: 0.5,
        wordCount: cleanedSnippet.split(/\s+/).length,
        metadata: {
          snippet: result.snippet,
          extraction_method: 'snippet_only',
        },
      };
    }
    return resultData;
  });

  const filteredResults = savedResults.filter(r => r !== null);
  console.log(`[DuckDuckGo] Returning ${filteredResults.length} results`);
  return filteredResults;
}

module.exports = { searchDuckDuckGo };
