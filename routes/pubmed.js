const { pubmedController } = require('../controller/pubmedController');

const router = require('express').Router();

router.post('/pubmed', pubmedController);

module.exports = router;
