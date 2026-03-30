const router = require("express").Router();
const { youtubeSearchController } = require("../controller/youtubeController");

router.post("/youtube", youtubeSearchController);

module.exports = router;
