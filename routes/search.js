const router = require("express").Router();
const { searchController } = require("../controller/duckduckgoController");

router.post("/search", searchController);

module.exports = router;
