const { searchMedRxivPost } = require("../controller/medrxivController");

const router = require("express").Router();

router.post("/search/medrxiv", searchMedRxivPost);

module.exports = router;
