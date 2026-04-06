const { hackerNewsController } = require("../controller/hackerNewsController");

const router = require("express").Router();


router.post("/hackernew", hackerNewsController)

module.exports = router;