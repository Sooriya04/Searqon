const { searchPubMed } = require('../services/pubmed');

async function pubmedController(req, res) {
  try {
    const { query, limit } = req.body || {};

    if (!query) {
      return res.status(400).json({
        error: "Query parameter 'query' is required",
      });
    }

    const maxResults = limit ? parseInt(limit, 10) : 10;
    const result = await searchPubMed(query, maxResults);

    return res.json({
      source: 'PubMed search',
      query: query,
      count: result.count,
      results: result.results,
    });
  } catch (err) {
    return res.status(500).json({
      error: 'PubMed search failed',
      message: err.message,
    });
  }
}

module.exports = {
  pubmedController,
};
