const { searchWithReadmes } = require("../services/github");


async function githubSearchWithReadmeController(req, res) {
  try {
    const { q } = req.body;

    if (!q) {
      return res.status(400).json({
        error: "Query parameter 'q' is required"
      });
    }

    const results = await searchWithReadmes(q);

    return res.json({
      source: "github",
      query: q,
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
