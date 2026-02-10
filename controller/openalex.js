const { searchOpenAlex } = require("../services/openalex");

const openAlexController = async (req, res) => {
  try {
    const { query, limit } = req.body || {};

    if (!query) {
      return res.status(400).json({
        success: false,
        error: "Query parameter 'query' is required",
      });
    }

    const maxResults = limit ? parseInt(limit, 10) : 10;

    const data = await searchOpenAlex(query, maxResults);

    return res.json({
      success: true,
      ...data,
    });
  } catch (err) {
    return res.status(500).json({
      success: false,
      message: err.message,
    });
  }
}
module.exports = {
  openAlexController
}