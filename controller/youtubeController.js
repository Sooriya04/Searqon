const { searchYoutube } = require("../services/youtube");

async function youtubeSearchController(req, res) {
  const { q, maxResults = 5 } = req.body;

  // Validate input
  if (!q || typeof q !== "string" || q.trim().length === 0) {
    return res.status(400).json({
      error: "Invalid request body",
      message: "`q` must be a non-empty string",
    });
  }

  try {
    const startTime = Date.now();
    const results = await searchYoutube(q.trim(), maxResults);
    const responseTime = Date.now() - startTime;
    
    return res.status(200).json({
      query: q.trim(),
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
