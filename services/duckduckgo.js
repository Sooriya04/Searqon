const cheerio = require("cheerio");
const httpClient = require("../utils/httpClient");
const { BROWSER_HEADERS } = require("../utils/browserHeaders");
const { cleanText, cleanSearchSnippet } = require("../utils/textCleaner");
const { extractPageContent } = require("../utils/contentExtractor");
const Result = require("../models/result");

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

async function searchDuckDuckGo(query, limit = 5) {
  console.log(`[DuckDuckGo] Searching for: "${query}"`);
  
  const rawResults = await fetchSearchResults(query);
  const savedResults = [];

  for (const result of rawResults) {
    if (savedResults.length >= limit) break;

    let resultData = null;

    // Try to extract full page content
    try {
      console.log(`[DuckDuckGo] Extracting content from: ${result.url}`);
      const pageData = await extractPageContent(result.url);

      // Use extracted content if quality is good
      if (pageData && pageData.content && pageData.wordCount >= 50) {
        console.log(`[DuckDuckGo] Extracted ${pageData.wordCount} words`);
        resultData = {
          query: query,
          source: "duckduckgo",
          title: pageData.title || result.title,
          url: result.url,
          content: pageData.content,
          rawContent: pageData.content,
          score: pageData.score,
          wordCount: pageData.wordCount,
          publishedDate: pageData.publishedDate || null,
          author: pageData.author || null,
        };
      }
    } catch (err) {
      console.log(`[DuckDuckGo] Extraction failed: ${err.message}`);
    }

    // FALLBACK: Use snippet if extraction failed
    if (!resultData && result.snippet && result.snippet.length >= 30) {
      const cleanedSnippet = cleanSearchSnippet(result.snippet);
      if (cleanedSnippet.length >= 30) {
        console.log(`[DuckDuckGo] Using snippet for ${result.url}`);
        resultData = {
          query: query,
          source: "duckduckgo",
          title: result.title,
          url: result.url,
          content: cleanedSnippet,
          rawContent: cleanedSnippet,
          score: 0.5,
          wordCount: cleanedSnippet.split(/\s+/).length,
        };
      }
    }

    // Save to MongoDB if we have valid data
    if (resultData) {
      const savedResult = new Result(resultData);
      await savedResult.save();
      console.log(`[DuckDuckGo] Saved result to database`);

      savedResults.push({
        query: resultData.query,
        source: resultData.source,
        title: resultData.title,
        url: resultData.url,
        content: resultData.content,
        wordCount: resultData.wordCount,
        score: resultData.score,
      });
    }
  }

  console.log(`[DuckDuckGo] Returning ${savedResults.length} results`);
  return savedResults;
}

module.exports = { searchDuckDuckGo };
