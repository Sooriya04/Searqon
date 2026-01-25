const cheerio = require("cheerio");
const axios = require("axios");
const { cleanText } = require("../utils/textCleaner");
const Result = require("../models/result");

const WIKI_SEARCH_API = "https://en.wikipedia.org/w/api.php";
const WIKI_EXTRACT_URL = "https://en.wikipedia.org/api/rest_v1/page/html";


async function searchWikiTitle(query) {
  try {
    const response = await axios.get(WIKI_SEARCH_API, {
      params: {
        action: "opensearch",
        search: query,
        limit: 1,
        format: "json",
      },
      headers: {
        "User-Agent": "SearqonBot/1.0",
      },
    });

    const titles = response.data[1];
    return titles && titles.length > 0 ? titles[0] : null;
  } catch (err) {
    console.error(`[Wiki] Search failed: ${err.message}`);
    return null;
  }
}


async function extractWikiContent(title) {
  const url = `${WIKI_EXTRACT_URL}/${encodeURIComponent(title)}`;

  const response = await axios.get(url, {
    headers: {
      "User-Agent": "SearqonBot/1.0",
    },
    timeout: 10000,
  });

  const html = response.data;

  // Parse HTML with cheerio
  const $ = cheerio.load(html);

  // Remove unwanted elements
  $("script, style, nav, footer, .mw-editsection, .reference, sup, .infobox, .navbox, .sidebar").remove();

  // Extract paragraphs
  const paragraphs = [];
  $("p").each((i, el) => {
    const text = cleanText($(el).text());
    if (text && text.length > 20) {
      paragraphs.push(text);
    }
  });

  // Join paragraphs with double newlines
  const content = paragraphs.join("\n\n");

  return {
    title,
    content,
    url: `https://en.wikipedia.org/wiki/${encodeURIComponent(title)}`,
    wordCount: content.split(/\s+/).length,
  };
}


async function wikiSearch(query) {
  console.log(`[Wiki] Searching for: "${query}"`);

  // First, find the correct article title
  const title = await searchWikiTitle(query);

  if (!title) {
    throw new Error(`No Wikipedia article found for "${query}"`);
  }

  console.log(`[Wiki] Found article: "${title}"`);

  // Extract content from the article
  const extracted = await extractWikiContent(title);

  console.log(`[Wiki] Extracted ${extracted.wordCount} words from "${title}"`);

  // Save to MongoDB
  const result = new Result({
    query: query,
    source: "wikipedia",
    title: extracted.title,
    url: extracted.url,
    content: extracted.content,
    rawContent: extracted.content,
    wordCount: extracted.wordCount,
    score: 0.9,
  });

  await result.save();
  console.log(`[Wiki] Saved result to database`);

  return {
    query: query,
    source: "wikipedia",
    title: result.title,
    url: result.url,
    content: result.content,
    wordCount: result.wordCount,
    score: result.score,
  };
}

module.exports = {
  wikiSearch,
};
