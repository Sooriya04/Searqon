const { searchYoutube } = require("../services/youtube");

async function youtubeSearchController(req, res) {
  const { query, maxResults = 5 } = req.body;

  // Validate input
  if (!query || typeof query !== "string" || query.trim().length === 0) {
    return res.status(400).json({
      error: "Invalid request body",
      message: "`query` must be a non-empty string",
    });
  }

  try {
    const startTime = Date.now();
    const results = await searchYoutube(query.trim(), maxResults);
    const responseTime = Date.now() - startTime;
    
    return res.status(200).json({
      query: query.trim(),
      responseTime,
      results: results.map((r) => ({
        title: r.title,
        url: r.url,
        content: r.content,
        description: r.description,
        score: r.score,
        wordCount: r.wordCount,
        metadata: r.metadata,
      })),
    });
  } catch (err) {
    console.error("[YouTube] Search execution failed:", err.message);

    return res.status(500).json({
      error: "Search failed",
      reason: err.message,
    });
  }
}

module.exports = { youtubeSearchController };
