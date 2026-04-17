
const { searchDuckDuckGo } = require("../services/duckduckgo");

async function searchController(req, res) {
  const { query, maxResults = 5, includeRawContent = true } = req.body;

  // Validate input
  if (!query || typeof query !== "string" || query.trim().length === 0) {
    return res.status(400).json({
      error: "Invalid request body",
      message: "`query` must be a non-empty string",
    });
  }

  try {
    const startTime = Date.now();
    const results = await searchDuckDuckGo(query.trim(), maxResults);
    const responseTime = Date.now() - startTime;
    
    return res.status(200).json({
      success: true,
      query:   query.trim(),
      results: results.map((r) => ({
        title: r.title,
        url:   r.url,
        content: r.content,
        rawContent: includeRawContent ? r.rawContent : undefined,
        score: r.score,
        metadata: {
          publishedDate: r.publishedDate || null,
          author: r.author || null,
          duration: `${responseTime}ms`
        }
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
