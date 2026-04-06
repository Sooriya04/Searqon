const { unifiedSearchPost } = require("../controller/unifiedController");

const router = require("express").Router();

router.post("/", unifiedSearchPost);

module.exports = router;
