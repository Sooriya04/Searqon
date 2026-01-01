const { searchDuckDuckGo } = require("../services/duckduckgo");

async function searchController(req, res) {
  const { q } = req.body;

  // Validate input
  if (!q || typeof q !== "string" || q.trim().length === 0) {
    return res.status(400).json({
      error: "Invalid request body",
      message: "`q` must be a non-empty string"
    });
  }

  try {

    const results = await searchDuckDuckGo(q.trim());

    return res.status(200).json({
      query: q,
      count: results.length,
      results
    });
  } catch (err) {
    console.error("Search execution failed", {
      query: q,
      error: err.message
    });

    return res.status(500).json({
      error: "Search failed",
      reason: err.message
    });
  }
}

module.exports = {
  searchController
};
