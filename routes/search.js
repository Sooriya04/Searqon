const router = require("express").Router();
const { searchController } = require("../controller/duckduckgoController");

router.post("/", searchController);

module.exports = router;
