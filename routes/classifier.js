const { classifyPost } = require("../controller/classifierController");

const router = require("express").Router();

router.post("/", classifyPost);

module.exports = router;
