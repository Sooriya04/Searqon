const { openAlexController } = require("../controller/openAlexController")

const router = require("express").Router()

router.post("/search/openalex", openAlexController)


module.exports = router