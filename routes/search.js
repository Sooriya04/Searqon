const express = require("express");
const { searchController } = require("../controller/searchController");

const router = express.Router();

router.post("/search", searchController);

module.exports = router;
