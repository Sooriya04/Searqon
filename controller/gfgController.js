const { searchGeeksForGeeks } = require("../services/geeksforgeeks");

async function gfgController(req, res) {
    const { query, limit } = req.body;

    if (!query || typeof query !== "string" || query.trim().length === 0) {
        return res.status(400).json({
            success: false,
            message: "Query must be a non-empty string",
        });
    }

    const maxResults = limit ? parseInt(limit, 10) : 5;

    try {
        const startTime = Date.now();
        const results = await searchGeeksForGeeks(query.trim(), maxResults);
        const duration = Date.now() - startTime;

        return res.status(200).json({
            success: true,
            query: query.trim(),
            count: results.length,
            timing: {
                duration: `${duration}ms`,
            },
            results: results,
        });
    } catch (error) {
        console.error(`[GFG Controller] Error: ${error.message}`);
        return res.status(500).json({
            success: false,
            message: error ? error.message : "Unknown error",
        });
    }
}

module.exports = { gfgController };
