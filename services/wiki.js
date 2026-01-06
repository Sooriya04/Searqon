const WIKI_EXTRACT_URL =
  "https://en.wikipedia.org/api/rest_v1/page/html";

async function wikiSearch(title, limit = 5) {
  const url = `${WIKI_SEARCH_URL}?q=${encodeURIComponent(query)}&limit=${limit}`;

  const response = await fetch(url, {
    headers: {
      "User-Agent": "SearqonBot/1.0"
    }
  });

  if (!response.ok) {
    throw new Error("Failed to extract Wikipedia page");
  }

  const html = await response.text();

  return {
    title,
    source: "wikipedia",
    content_html: html,
    url: `https://en.wikipedia.org/wiki/${encodeURIComponent(title)}`
  };
}

module.exports = {
  wikiSearch
};
