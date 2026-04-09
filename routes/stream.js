const express = require('express');
const router = express.Router();
const streamController = require('../controller/streamController');

router.get('/', streamController.streamSearchGet);

module.exports = router;
