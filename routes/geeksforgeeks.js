const express = require('express');
const router = express.Router();
const { gfgController } = require('../controller/gfgController');

router.post('/geeksforgeeks', gfgController);

module.exports = router;
