const cheerio = require("cheerio");
const httpClient = require("../utils/httpClient");
const { cleanText } = require("../utils/textCleaner");

/**
 * Fetch and extract content from a URL
 * @param {string} url - URL to fetch
 * @returns {Promise<string>} Extracted text content
 */
async function fetchPageContent(url) {
  try {
    const response = await httpClient.get(url, {
      timeout: 8000,
      headers: {
        Accept: "text/html",
        "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/121.0 Safari/537.36"
      }
    });

    const $ = cheerio.load(response.data);

    // Remove unwanted elements
    $("script, style, nav, footer, header, aside, iframe, noscript, .ads, .comments, .sidebar").remove();

    // Extract text from main content areas
    const paragraphs = [];
    $("article p, main p, .content p, .post p, p").each((i, el) => {
      const text = cleanText($(el).text());
      if (text && text.length > 30) {
        paragraphs.push(text);
      }
    });

    // Return all extracted paragraphs
    return paragraphs.join("\n\n") || "Content could not be extracted";
  } catch (err) {
    return "Content could not be fetched";
  }
}

async function searchHNByQuery(query) {
  // Use the Algolia HN Search API which returns JSON directly
  const searchUrl = `https://hn.algolia.com/api/v1/search?query=${encodeURIComponent(
    query
  )}&tags=story&hitsPerPage=10`;

  const response = await httpClient.get(searchUrl, {
    headers: {
      Accept: "application/json"
    }
  });

  const results = [];

  if (response.data && response.data.hits) {
    // Fetch content for all URLs in parallel
    const contentPromises = response.data.hits.map(async (hit) => {
      const title = hit.title || "";
      const url = hit.url || `https://news.ycombinator.com/item?id=${hit.objectID}`;
      
      // Fetch actual page content
      const content = await fetchPageContent(url);

      if (title) {
        return {
          title,
          url,
          content,
          points: hit.points || 0,
          author: hit.author || "unknown",
          created_at: hit.created_at || null
        };
      }
      return null;
    });

    const fetchedResults = await Promise.all(contentPromises);
    results.push(...fetchedResults.filter(r => r !== null));
  }

  return results;
}

module.exports = {
    searchHNByQuery
}