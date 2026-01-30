const axios = require("axios");
const cheerio = require("cheerio");

async function searchPubMed(query, limit = 10) {
  if (!query) {
    throw new Error("Query is required");
  }

  const url = `https://pubmed.ncbi.nlm.nih.gov/?term=${encodeURIComponent(
    query
  )}`;

  try {
    const response = await axios.get(url, {
      headers: {
        "User-Agent":
          "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120 Safari/537.36",
        Accept: "text/html",
      },
      timeout: 15000,
    });

    const $ = cheerio.load(response.data);
    const results = [];

    $(".docsum-content").each((i, el) => {
      if (i >= limit) return false;

      const title = $(el).find(".docsum-title").text().trim();
      const relativeLink = $(el).find(".docsum-title").attr("href");
      const link = relativeLink
        ? `https://pubmed.ncbi.nlm.nih.gov${relativeLink}`
        : null;

      const authors = $(el)
        .find(".docsum-authors")
        .text()
        .trim();

      const journalInfo = $(el)
        .find(".docsum-journal-citation")
        .text()
        .trim();

      const snippet = $(el)
        .find(".full-view-snippet")
        .text()
        .trim();

      if (title && link) {
        results.push({
          source: "pubmed",
          title,
          link,
          authors,
          journal: journalInfo,
          snippet,
        });
      }
    });

    return {
      success: true,
      query,
      count: results.length,
      data: results,
    };
  } catch (error) {
    return {
      success: false,
      query,
      message: "Failed to fetch PubMed results",
      error: error.message,
    };
  }
}
(async () => {
  const result = await searchPubMed("agentic ai", 5);
  console.log(JSON.stringify(result, null, 2));
})();