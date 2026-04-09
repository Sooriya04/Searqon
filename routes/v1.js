const express = require('express');
const router = express.Router();

// ─── Import Core Features ───────────────────────────────────────────────────
const crawlRoutes  = require('./crawl');
const searchRoutes = require('./search');
const unified      = require('./unified');
const classifier   = require('./classifier');
const chatRoutes   = require('./chat');

// ─── Import Specialized Sources ──────────────────────────────────────────────
const reddit         = require('./reddit');
const wiki           = require('./wiki');
const github         = require('./github');
const hackernew      = require('./hackernew');
const arxiv          = require('./arxiv');
const pubmed         = require('./pubmed');
const openalex       = require('./openalex');
const doaj           = require('./doaj');
const medrxiv        = require('./medrxiv');
const geeksforgeeks  = require('./geeksforgeeks');
const youtube        = require('./youtube');

// ─── Versioned Mounting (v1) ────────────────────────────────────────────────

// Core endpoints
router.use('/crawl',      crawlRoutes);
router.use('/scrape',     crawlRoutes); // scrape is handled by crawl router
router.use('/search',     searchRoutes);
router.use('/unified',    unified);
router.use('/classify',   classifier);
router.use('/chat',       chatRoutes);

// Source search endpoints
// These are currently mounted under /api/search/... so we'll maintain that or move to /v1/sources/...
// Based on "production level", grouping them is better.
router.use('/sources', reddit);
router.use('/sources', wiki);
router.use('/sources', github);
router.use('/sources', hackernew);
router.use('/sources', arxiv);
router.use('/sources', pubmed);
router.use('/sources', openalex);
router.use('/sources', doaj);
router.use('/sources', medrxiv);
router.use('/sources', geeksforgeeks);
router.use('/sources', youtube);

module.exports = router;
