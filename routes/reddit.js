const router = require("express").Router();

const { redditController } = require("../controller/redditController");

router.post("/search/reddit", redditController);

module.exports = router;