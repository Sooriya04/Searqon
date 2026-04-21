const express = require('express');
const router = express.Router();

// ─── Import Core Controllers ───────────────────────────────────────────────
const {
    scrape,
    map,
    crawl,
    getCrawlStatus,
    extract
} = require('../controller/crawlController');
const { searchController } = require('../controller/duckduckgoController');
const { unifiedSearchPost } = require('../controller/unifiedController');

// ─── Import Specialized Sources Controllers ────────────────────────────────
const { redditController } = require('../controller/redditController');
const { githubSearchWithReadmeController } = require('../controller/githubController');
const { wikiController } = require('../controller/wikiController');
const { hackerNewsController } = require('../controller/hackerNewsController');
const { searchArxivPost } = require('../controller/arxivController');
const { pubmedController } = require('../controller/pubmedController');
const { youtubeSearchController } = require('../controller/youtubeController');

// ─── High-Level Orchestration ──────────────────────────────────────────────
// POST /api/v2/research -> The unified extraction pipeline
router.post('/research', unifiedSearchPost);

// ─── Core Tools ────────────────────────────────────────────────────────────
// POST /api/v2/search -> Discovery only
router.post('/search', searchController);

// Atomic data tools
router.post('/scrape', scrape);
router.post('/crawl', crawl);
router.get('/crawl/:id', getCrawlStatus);
router.post('/map', map);
router.post('/extract', extract);

// ─── Specialized Sources ───────────────────────────────────────────────────
router.post('/sources/reddit', redditController);
router.post('/sources/github', githubSearchWithReadmeController);
router.post('/sources/wikipedia', wikiController);
router.post('/sources/hackernews', hackerNewsController);
router.post('/sources/arxiv', searchArxivPost);
router.post('/sources/pubmed', pubmedController);
router.post('/sources/youtube', youtubeSearchController);

module.exports = router;
