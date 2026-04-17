const { reddit } = require("../services/reddit");

async function redditController(req, res) {
    try {
        const { query } = req.body;
        if(!query){
            return res.status(400).json({
                error : "Query parameter 'query' is required" 
            });
        }
        const results = await reddit(query);

        return res.json({
            source : "reddit",
            query : query,
            count : results.length,
            results
        })
    }catch(err){
        return res.status(400).json({
            error : "Reddit search failed",
            message  : err.message
        })
    }
}

module.exports = {
    redditController
}