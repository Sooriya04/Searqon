const cheerio = require("cheerio");
const httpClient = require("../utils/httpClient");
const { BROWSER_HEADERS } = require("../utils/browserHeaders");
const { cleanText, cleanSearchSnippet } = require("../utils/textCleaner");
const { extractPageContent } = require("../utils/contentExtractor");

const DUCKDUCKGO_URL = "https://html.duckduckgo.com/html/";

function isAdUrl(url) {
  return url && url.includes("duckduckgo.com/y.js");
}

async function fetchSearchResults(query) {
  const response = await httpClient.post(
    DUCKDUCKGO_URL,
    new URLSearchParams({ q: query }).toString(),
    {
      headers: {
        ...BROWSER_HEADERS,
        "Content-Type": "application/x-www-form-urlencoded",
      },
      responseType: "text",
    }
  );

  if (!response || typeof response.data !== "string") {
    throw new Error("DuckDuckGo returned no HTML");
  }

  return parseSearchResults(response.data);
}


function parseSearchResults(html) {
  const $ = cheerio.load(html);
  const results = [];

  $(".result").each((i, el) => {
    const $el = $(el);
    const link = $el.find(".result__a");
    const title = cleanText(link.text());
    const url = link.attr("href");
    const snippet = cleanText($el.find(".result__snippet").text());

    if (title && url && !isAdUrl(url)) {
      results.push({ title, url, snippet });
    }
  });

  return results;
}


function buildResult(result, pageData) {
  if (pageData && pageData.content && pageData.wordCount >= 50) {
    return {
      title: pageData.title || result.title,
      url: result.url,
      content: pageData.content,
      score: pageData.score,
      publishedDate: pageData.publishedDate || null,
      author: pageData.author || null,
    };
  }

  // Fallback to snippet (cleaned)
  if (result.snippet && result.snippet.length >= 50) {
    const cleanedSnippet = cleanSearchSnippet(result.snippet);
    if (cleanedSnippet.length >= 30) {
      return {
        title: result.title,
        url: result.url,
        content: cleanedSnippet,
        rawContent: cleanedSnippet,
        score: 0.5,
        publishedDate: null,
        author: null,
      };
    }
  }

  return null;
}

async function searchDuckDuckGo(query, limit = 5) {
  const rawResults = await fetchSearchResults(query);
  const results = [];

  for (const result of rawResults) {
    if (results.length >= limit) break;

    // Extract full page content only - no snippet fallbacks
    try {
      const pageData = await extractPageContent(result.url);

      // Only include results with quality content
      if (pageData && pageData.content && pageData.wordCount >= 100) {
        results.push({
          title: pageData.title || result.title,
          url: result.url,
          content: pageData.content,
          rawContent: pageData.content,
          score: pageData.score,
          publishedDate: pageData.publishedDate || null,
          author: pageData.author || null,
        });
      }
    } catch {
      // Skip failed extractions - no snippet fallback
    }
  }

  return results;
}

module.exports = { searchDuckDuckGo };
