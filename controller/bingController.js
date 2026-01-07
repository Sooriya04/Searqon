/**
 * Bing Search Controller
 * Handles Bing search requests and returns Tavily-style responses
 */

const { searchBing } = require("../services/bing");

/**
 * Handle Bing search request
 * @param {Request} req - Express request
 * @param {Response} res - Express response
 */
async function bingController(req, res) {
  const { q, maxResults = 5, includeRawContent = true } = req.body;

  // Validate input
  if (!q || typeof q !== "string" || q.trim().length === 0) {
    return res.status(400).json({
      error: "Query parameter 'q' is required",
    });
  }

  try {
    const startTime = Date.now();
    const results = await searchBing(q.trim(), maxResults);
    const responseTime = Date.now() - startTime;

    // Tavily-style response format
    return res.status(200).json({
      engine: "bing",
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
    console.error("Bing search failed:", err.message);

    return res.status(500).json({
      error: "Bing search failed",
      reason: err.message,
    });
  }
}

module.exports = { bingController };