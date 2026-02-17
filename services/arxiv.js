const axios = require('axios');
const xml2js = require('xml2js');
const ScrapUrl = require('../scrapper/ScrapUrl');

const ARXIV_API = 'http://export.arxiv.org/api/query';

async function searchArxiv(query, limit = 10) {
  console.log(`[Arxiv] Searching for: "${query}"`);

  if (!query || typeof query !== 'string') {
    throw new Error('Valid query is required');
  }

  const response = await axios.get(ARXIV_API, {
    params: {
      search_query: `all:${query}`,
      start: 0,
      max_results: limit,
      sortBy: 'relevance',
      sortOrder: 'descending',
    },
  });

  const parsed = await xml2js.parseStringPromise(response.data, {
    explicitArray: false,
  });

  const entries = parsed.feed?.entry || [];
  const list = Array.isArray(entries) ? entries : [entries];
  const savedResults = [];

  for (const item of list) {
    const title = item.title?.trim();
    const summary = item.summary?.trim();
    const authors = item.author
      ? Array.isArray(item.author)
        ? item.author.map((a) => a.name)
        : [item.author.name]
      : [];

    // Get the abstract page URL for scraping
    const arxivUrl = item.id; // e.g. http://arxiv.org/abs/2301.12345v1

    if (title && summary && summary.length >= 50) {
      let content = summary;

      // Try to scrape the full arxiv page for richer content
      if (arxivUrl) {
        try {
          console.log(`[Arxiv] Scraping: ${arxivUrl}`);
          const scraped = await ScrapUrl(arxivUrl);
          if (scraped && scraped.content && scraped.content.length > summary.length) {
            content = scraped.content;
          }
        } catch (e) {
          console.log(`[Arxiv] Scraping failed for ${arxivUrl}: ${e.message}`);
        }
      }

      savedResults.push({
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
      });
    }
  }

  console.log(`[Arxiv] Returning ${savedResults.length} results`);
  return savedResults;
}

module.exports = { searchArxiv };
