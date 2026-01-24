
const { searchDuckDuckGo } = require("../services/duckduckgo");

async function searchController(req, res) {
  const { q, maxResults = 5, includeRawContent = true } = req.body;

  // Validate input
  if (!q || typeof q !== "string" || q.trim().length === 0) {
    return res.status(400).json({
      error: "Invalid request body",
      message: "`q` must be a non-empty string",
    });
  }

  try {
    const startTime = Date.now();
    const results = await searchDuckDuckGo(q.trim(), maxResults);
    const responseTime = Date.now() - startTime;

    // Tavily-style response format
    return res.status(200).json({
      query: q.trim(),
      responseTime,
      results: results.map((r) => ({
        title: r.title,
        url: r.url,
        content: r.content,
        rawContent: includeRawContent ? r.rawContent : undefined,
        score: r.score,
        publishedDate: r.publishedDate || null,
        author: r.author || null,
      })),
    });
  } catch (err) {
    console.error("Search execution failed:", err.message);

    return res.status(500).json({
      error: "Search failed",
      reason: err.message,
    });
  }
}

module.exports = { searchController };
