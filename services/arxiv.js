const axios = require("axios");
const xml2js = require("xml2js");

const ARXIV_API = "http://export.arxiv.org/api/query";

async function searchArxiv(query) {
  if (!query || typeof query !== "string") {
    throw new Error("Valid query is required");
  }

  const response = await axios.get(ARXIV_API, {
    params: {
      search_query: `all:${query}`,
      start: 0,
      max_results: 10,
      sortBy: "relevance",
      sortOrder: "descending"
    }
  });

  const parsed = await xml2js.parseStringPromise(response.data, {
    explicitArray: false
  });

  const entries = parsed.feed?.entry || [];
  const list = Array.isArray(entries) ? entries : [entries];

  return list.map(item => ({
    source: "arxiv",
    title: item.title?.trim(),
    summary: item.summary?.trim(),
    authors: item.author
      ? Array.isArray(item.author)
        ? item.author.map(a => a.name)
        : [item.author.name]
      : [],
    published: item.published,
    updated: item.updated,
    link: item.id
  }));
}

module.exports = { searchArxiv };