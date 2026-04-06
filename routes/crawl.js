const express = require('express');
const router = express.Router();
const {
    scrape,
    map,
    crawl,
    getCrawlStatus,
    search,
    extract
} = require('../controller/crawlController');

// POST /api/v1/scrape  → Single URL scrape (markdown, text, or html)
router.post('/scrape', scrape);

// POST /api/v1/map     → Discover all internal URLs of a domain
router.post('/map', map);

// POST /api/v1/crawl   → Recursively crawl and scrape a whole site (Returns Job ID)
router.post('/crawl', crawl);

// GET /api/v1/crawl/:id → Get status and results for a crawl job
router.get('/crawl/:id', getCrawlStatus);

// POST /api/v1/search  → Single query to search and scrape results
router.post('/search', search);

// POST /api/v1/extract → Schema-driven structured JSON extraction via LLM
router.post('/extract', extract);

module.exports = router;
