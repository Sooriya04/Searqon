const { wikiSearch } = require("../services/wiki");

async function wikiController(req, res) {
    try {
        const { query } = req.body;
        if (!query) {
            return res
                .status(400)
                .json({
                    error: "Query parameter 'query' is required"
                });
        }
        const result = await wikiSearch(query);

        return res.json({
            source: "Wiki search",
            query: query,
            count: 1,
            result
        })
    } catch (err) {
        return res
            .status(500)
            .json({
                error: "Wikipedia search failed",
                message: err.message
            })
    }
}

module.exports = {
    wikiController
}