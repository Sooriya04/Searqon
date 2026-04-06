const { githubSearchWithReadmeController } = require("../controller/githubController");

const router = require("express").Router();

router.post("/github", githubSearchWithReadmeController)

module.exports = router;