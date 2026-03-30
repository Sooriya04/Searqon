/**
 * Classifier Controller
 * Handles POST /api/classify — exposes the Python classifier as a REST endpoint.
 */

const { routeQuery } = require("../services/classifierService");

/**
 * POST /api/classify
 * Body: { query: string }
 * Returns: { query, strategy, categories, sources }
 */
exports.classifyPost = async (req, res) => {
    try {
        const { query } = req.body;

        if (!query || typeof query !== "string" || query.trim().length === 0) {
            return res.status(400).json({
                success: false,
                message: "query must be a non-empty string",
            });
        }

        const result = await routeQuery(query.trim());

        return res.json({
            success: true,
            query: query.trim(),
            strategy:   result.strategy,
            categories: result.categories,
            sources:    result.sources,
        });
    } catch (error) {
        console.error(`[ClassifierController] Error: ${error.message}`);
        return res.status(500).json({
            success: false,
            message: error.message,
        });
    }
};
