const { searchArxiv } = require("../services/arxiv");

exports.searchArxivPost = async (req, res) => {
  try {
    const { query } = req.body;

    if (!query) {
      return res.status(400).json({
        success: false,
        message: "Query is required"
      });
    }

    const data = await searchArxiv(query);

    res.json({
      success: true,
      query,
      count: data.length,
      data
    });
  } catch (error) {
    res.status(500).json({
      success: false,
      message: error.message
    });
  }
};