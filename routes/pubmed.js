const { pubmedController } = require('../controller/pubmedController');

const router = require('express').Router();

router.post('/search/pubmed', pubmedController);

module.exports = router;
