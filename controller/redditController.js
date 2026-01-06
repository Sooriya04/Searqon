const { reddit } = require("../services/reddit");

async function redditController(req, res) {
    try {
        const {q} = req.body;
        if(!q){
            return res.status(400).json({
                error : "Query parameter 'q' is required" 
            });
        }
        const results = await reddit(q);

        return res.json({
            source : "reddit",
            query : q,
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