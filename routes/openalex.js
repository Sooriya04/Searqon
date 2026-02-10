const { openAlexController } = require("../controller/openalex")

const router = require("express").Router()

router.post("/search/openalex", openAlexController)


module.exports = router