const router = require("express").Router();

const { redditController } = require("../controller/redditController");

router.post("/reddit", redditController);

module.exports = router;