const cheerio = require("cheerio");
const httpClient = require("../utils/httpClient");

const DUCKDUCKGO_URL = "https://html.duckduckgo.com/html/";

function isAdUrl(url) {
  return url.includes("duckduckgo.com/y.js");
}

async function searchDuckDuckGo(query) {
  const response = await httpClient.post(
    DUCKDUCKGO_URL,
    new URLSearchParams({ q: query }).toString(),
    {
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        "Accept": "text/html"
      },
      responseType: "text"
    }
  );

  if (!response || typeof response.data !== "string") {
    throw new Error("DuckDuckGo returned no HTML");
  }

  const $ = cheerio.load(response.data);
  const results = [];

  $(".result").each((_, el) => {
    const link = $(el).find(".result__a");
    const title = link.text();
    const url = link.attr("href");

    const snippet = $(el)
      .find(".result__snippet, .result__body")
      .text()
      .replace(/\s+/g, " ")
      .trim();

    if (title && url && !isAdUrl(url)) {
      results.push({
        title: title.trim(),
        url,
        snippet,
        source: "duckduckgo"
      });
    }
  });

  return results.slice(0, 10);
}

module.exports = { searchDuckDuckGo };
