const express = require('express');
const router = express.Router();
const {
    scrape,
    map,
    crawl,
    search,
    extract
} = require('../controller/crawlController');

// POST /api/scrape  → Single URL scrape (markdown, text, or html)
router.post('/scrape', scrape);

// POST /api/map     → Discover all internal URLs of a domain
router.post('/map', map);

// POST /api/crawl   → Recursively crawl and scrape a whole site
router.post('/crawl', crawl);

// POST /api/search  → Single query to search and scrape results
router.post('/search', search);

// POST /api/extract → Schema-driven structured JSON extraction via LLM
router.post('/extract', extract);

module.exports = router;
