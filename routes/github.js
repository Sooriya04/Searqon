const { githubSearchWithReadmeController } = require("../controller/githubController");

const router = require("express").Router();

router.post("/search/github", githubSearchWithReadmeController)

module.exports = router;