const pool = require('./pool');

/**
 * Scrapes a URL using the worker pool.
 * @param {string} url - The URL to scrape.
 * @returns {Promise<object>} - The scrape result.
 */
async function ScrapUrl(url) {
  if (!url) {
    throw new Error('URL is required');
  }
  return pool.schedule(url);
}

module.exports = ScrapUrl;
