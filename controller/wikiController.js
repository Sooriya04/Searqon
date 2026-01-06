const { wikiSearch } = require("../services/wiki");

async function wikiController(req, res) {
    try{
        const {q} = req.body;
        if(!q){
            return res
            .status(400)
            .json({
                error : "Query parameter 'q' is required"
            });
        }
        const result = await wikiSearch(q);

        return res.json({
            source : "Wiki search",
            query : q,
            count : result.length,
            result
        })
    }catch(err){
        return res
        .status(500)
        .json({
            error : "Wikipedia search failed",
            message : err.message
        })
    }
}

module.exports = {
    wikiController
}