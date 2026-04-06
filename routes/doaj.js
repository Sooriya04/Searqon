const { searchDOAJPost } = require("../controller/doajController");

const router = require("express").Router();

router.post("/doaj", searchDOAJPost);

module.exports = router;
