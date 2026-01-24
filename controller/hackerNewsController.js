const { searchHNByQuery } = require("../services/hackernews");

async function hackerNewsController(req, res){
    try{

        const { q } = req.body;
        const result = await searchHNByQuery(q);
        return res.status(200)
        .json({
            success : true,
            query : q,
            data : result
        })

    }catch(e){
        return res.status(500).json({
            success: false,
            message: e.message
        })
    }
}
module.exports = {
    hackerNewsController
}