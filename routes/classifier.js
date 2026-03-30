const { classifyPost } = require("../controller/classifierController");

const router = require("express").Router();

router.post("/classify", classifyPost);

module.exports = router;
