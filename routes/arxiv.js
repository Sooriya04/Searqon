const { searchArxivPost } = require("../controller/arxivController");

const router = require("express").Router();

router.post("/arxiv", searchArxivPost);

module.exports = router;