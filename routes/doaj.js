const { searchDOAJPost } = require("../controller/doajController");

const router = require("express").Router();

router.post("/search/doaj", searchDOAJPost);

module.exports = router;
