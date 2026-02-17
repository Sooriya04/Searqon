const { searchMedRxiv } = require("../services/medrxiv");

exports.searchMedRxivPost = async (req, res) => {
  try {
    const { query, limit } = req.body;

    if (!query || typeof query !== "string" || query.trim().length === 0) {
      return res.status(400).json({
        success: false,
        message: "Query must be a non-empty string",
      });
    }

    const maxResults = limit ? parseInt(limit, 10) : 5;
    const data = await searchMedRxiv(query, maxResults);

    res.json({
      success: true,
      query,
      count: data.length,
      data,
    });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: error.message,
    });
  }
};
