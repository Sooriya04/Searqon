const { wikiController } = require("../controller/wikiController");

const router = require("express").Router();

router.post("/search/wiki", wikiController);

module.exports = router;