const { searchWithReadmes } = require("../services/github");


async function githubSearchWithReadmeController(req, res) {
  try {
    const { query } = req.body;

    if (!query) {
      return res.status(400).json({
        error: "Query parameter 'q' is required"
      });
    }

    const results = await searchWithReadmes(query);

    return res.json({
      source: "github",
      query: query,
      count: results.length,
      results
    });

  } catch (err) {
    return res.status(500).json({
      error: "GitHub search with README failed",
      message: err.message
    });
  }
}

module.exports = {
  githubSearchWithReadmeController
};
