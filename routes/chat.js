const express = require('express');
const router = express.Router();
const chatController = require('../controller/chatController');

router.post('/', chatController.handleFollowUp);
router.post('/stream', chatController.handleFollowUpStream);

module.exports = router;
