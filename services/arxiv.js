const axios = require("axios");
const xml2js = require("xml2js");
const DatabaseClient = require("../database/client");

const ARXIV_API = "http://export.arxiv.org/api/query";

async function searchArxiv(query, limit = 10) {
  console.log(`[Arxiv] Searching for: "${query}"`);
  
  if (!query || typeof query !== "string") {
    throw new Error("Valid query is required");
  }

  const response = await axios.get(ARXIV_API, {
    params: {
      search_query: `all:${query}`,
      start: 0,
      max_results: limit,
      sortBy: "relevance",
      sortOrder: "descending"
    }
  });

  const parsed = await xml2js.parseStringPromise(response.data, {
    explicitArray: false
  });

  const entries = parsed.feed?.entry || [];
  const list = Array.isArray(entries) ? entries : [entries];
  const savedResults = [];

  for (const item of list) {
    const title = item.title?.trim();
    const summary = item.summary?.trim();
    const authors = item.author
      ? Array.isArray(item.author)
        ? item.author.map(a => a.name)
        : [item.author.name]
      : [];

    // Only save papers with meaningful content
    if (title && summary && summary.length >= 50) {
      const resultData = {
        query: query,
        source: "arxiv",
        title: title,
        url: item.id,
        content: summary,
        score: 0.8, // Arxiv papers are generally high quality
        wordCount: summary.split(/\s+/).length,
        author: authors.length > 0 ? authors[0] : "unknown",
        publishedDate: item.published,
        metadata: {
          authors: authors,
          updated: item.updated,
          categories: item.category ? (Array.isArray(item.category) ? item.category.map(c => c.$.term) : [item.category.$.term]) : [],
          arxiv_id: item.id
        }
      };

      // Save to database microservice
      await DatabaseClient.saveResult(resultData);
      console.log(`[Arxiv] Saved result to database: ${title}`);

      savedResults.push({
        query: resultData.query,
        source: resultData.source,
        title: resultData.title,
        summary: summary,
        authors: authors,
        published: item.published,
        updated: item.updated,
        link: item.id,
        wordCount: resultData.wordCount
      });
    }
  }

  console.log(`[Arxiv] Returning ${savedResults.length} results`);
  return savedResults;
}

module.exports = { searchArxiv };