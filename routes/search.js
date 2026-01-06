const router = require("express").Router();
const { searchController } = require("../controller/searchController");

router.post("/search", searchController);

module.exports = router;
