/**
 * Bing Search Service
 * Tavily-style search with full content extraction
 */

const cheerio = require("cheerio");
const httpClient = require("../utils/httpClient");
const { BROWSER_HEADERS } = require("../utils/browserHeaders");
const { cleanText, cleanSearchSnippet } = require("../utils/textCleaner");
const { extractPageContent } = require("../utils/contentExtractor");

const BING_URL = "https://www.bing.com/search";

/**
 * Extract real URL from Bing's redirect URL
 * Bing uses base64 encoded URLs in the 'u' parameter
 * @param {string} bingUrl - Bing redirect URL
 * @returns {string|null} Decoded URL or original
 */
function extractRealUrl(bingUrl) {
  if (!bingUrl) return null;

  // If it's already a direct URL, return as-is
  if (!bingUrl.includes("bing.com/ck/a")) {
    return bingUrl;
  }

  try {
    const urlObj = new URL(bingUrl);
    const encodedUrl = urlObj.searchParams.get("u");

    if (encodedUrl) {
      const base64Part = encodedUrl.replace(/^a1/, "");
      return Buffer.from(base64Part, "base64").toString("utf-8");
    }
  } catch {
    // Return original on decode failure
  }

  return bingUrl;
}

/**
 * Fetch search results from Bing
 * @param {string} query - Search query
 * @returns {Promise<string>} HTML response
 */
async function fetchBingResults(query) {
  const response = await httpClient.get(BING_URL, {
    params: {
      q: query,
      setlang: "en-us",
      cc: "US",
      first: 1,
    },
    headers: BROWSER_HEADERS,
  });

  if (!response || typeof response.data !== "string") {
    throw new Error("Bing returned no HTML");
  }

  return response.data;
}

/**
 * Parse search results from Bing HTML
 * @param {string} html - Bing results HTML
 * @returns {Array} Parsed results
 */
function parseSearchResults(html) {
  const $ = cheerio.load(html);
  const results = [];

  // Try multiple selectors for Bing results
  let items = $(".b_algo").toArray();
  if (items.length === 0) items = $("li.b_algo").toArray();
  if (items.length === 0) items = $("#b_results > li").toArray();

  for (const el of items) {
    const $el = $(el);
    const link = $el.find("h2 a").first();
    const title = cleanText(link.text());
    const rawUrl = link.attr("href");
    const url = extractRealUrl(rawUrl);
    const snippet = cleanText($el.find(".b_caption p, .b_algoSlug").text());

    if (title && url && !rawUrl.startsWith("javascript:")) {
      results.push({ title, url, rawUrl, snippet });
    }
  }

  // Fallback parsing if no results found
  if (results.length === 0) {
    $("ol#b_results li").each((i, el) => {
      const $el = $(el);
      const link = $el.find("h2 a, a.tilk").first();
      const title = cleanText(link.text());
      const rawUrl = link.attr("href");
      const url = extractRealUrl(rawUrl);
      const snippet = cleanText($el.find(".b_caption p, .b_algoSlug").text());

      if (title && url && snippet && !rawUrl.startsWith("javascript:")) {
        results.push({ title, url, rawUrl, snippet });
      }
    });
  }

  return results;
}

/**
 * Build result object with extracted content
 * @param {Object} result - Basic result info
 * @param {Object|null} pageData - Extracted page content
 * @returns {Object} Complete result object
 */
function buildResult(result, pageData) {
  // Use page data if available and quality
  if (pageData && pageData.content && pageData.wordCount >= 50) {
    return {
      title: pageData.title || result.title,
      url: result.url,
      content: pageData.content,
      rawContent: pageData.content,
      score: pageData.score,
      publishedDate: pageData.publishedDate || null,
      author: pageData.author || null,
      source: "bing",
    };
  }

  // Fallback to Bing snippet (cleaned)
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
        source: "bing",
      };
    }
  }

  return null;
}

/**
 * Search Bing and extract full page content (Tavily-style)
 * @param {string} query - Search query
 * @param {number} limit - Maximum results to return
 * @returns {Promise<Array>} Search results with extracted content
 */
async function searchBing(query, limit = 5) {
  const html = await fetchBingResults(query);
  const rawResults = parseSearchResults(html);
  const results = [];

  for (const result of rawResults) {
    if (results.length >= limit) break;

    // Extract full page content only - no snippet fallbacks
    try {
      const fetchUrl = result.url.startsWith("http") ? result.url : result.rawUrl;
      const pageData = await extractPageContent(fetchUrl);

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
          source: "bing",
        });
      }
    } catch {
      // Skip failed extractions - no snippet fallback
    }
  }

  return results;
}

module.exports = { searchBing };
