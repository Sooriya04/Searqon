const { bingController } = require("../controller/bingController");

const router = require("express").Router();

router.post("/search/bing", bingController)

module.exports = router;