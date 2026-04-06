const { wikiController } = require("../controller/wikiController");

const router = require("express").Router();

router.post("/wiki", wikiController);

module.exports = router;