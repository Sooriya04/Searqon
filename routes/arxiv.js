const { searchArxivPost } = require("../controller/arxivController");

const router = require("express").Router();

router.post("/search/arxiv", searchArxivPost);

module.exports = router;