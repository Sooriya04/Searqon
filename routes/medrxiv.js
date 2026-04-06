const { searchMedRxivPost } = require("../controller/medrxivController");

const router = require("express").Router();

router.post("/medrxiv", searchMedRxivPost);

module.exports = router;
