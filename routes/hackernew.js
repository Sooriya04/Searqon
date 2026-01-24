const { hackerNewsController } = require("../controller/hackerNewsController");

const router = require("express").Router();


router.post("/search/hackernew", hackerNewsController)

module.exports = router;