const { unifiedSearchPost } = require("../controller/unifiedController");

const router = require("express").Router();

router.post("/search/unified", unifiedSearchPost);

module.exports = router;
