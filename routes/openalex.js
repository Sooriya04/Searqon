const { openAlexController } = require("../controller/openAlexController")

const router = require("express").Router()

router.post("/openalex", openAlexController)


module.exports = router