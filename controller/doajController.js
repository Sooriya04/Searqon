const { searchDOAJ } = require("../services/doaj");

exports.searchDOAJPost = async (req, res) => {
    try {
        const { query, limit } = req.body;

        if (!query || typeof query !== "string" || query.trim().length === 0) {
            return res.status(400).json({
                success: false,
                message: "Query must be a non-empty string",
            });
        }

        const maxResults = limit ? parseInt(limit, 5) : 5;
        const data = await searchDOAJ(query, maxResults);

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