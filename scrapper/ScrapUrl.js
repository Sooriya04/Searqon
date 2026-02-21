const axios = require('axios');
const limiter = require("../utils/Limiter");
const config = require('../utils/configLoader');

const PYTHON_SCRAPER_URL = 'http://127.0.0.1:3002/scrape';
const BINARY_EXTENSIONS = /\.(jpg|jpeg|png|gif|pdf|zip|mp4|webp|svg)$/i;

async function ScrapUrl(url, options = {}) {
    if (BINARY_EXTENSIONS.test(url)) {
        console.warn(`[ScrapUrl] Skipping binary/image URL: ${url}`);
        return { title: "Binary File", content: "", url: url, wordCount: 0, duration: 0 };
    }

    return limiter.add(async () => {
        const startTime = Date.now();
        console.log(`[ScrapUrl] Delegating to Python API: ${url} at ${new Date(startTime).toISOString()}`);

        const timeout = options.timeout || config.browser.timeout || 15000;

        try {
            const response = await axios.post(PYTHON_SCRAPER_URL, { url: url }, {
                timeout: timeout + 5000, // Slightly higher than crawler timeout
                headers: { 'Content-Type': 'application/json' }
            });

            const report = response.data;

            console.log(`[ScrapUrl] Finished via API: ${url}`);
            console.log(`Title: ${report.title}`);
            console.log(`Duration: ${report.duration}ms`);

            return report;
        } catch (error) {
            if (error.response && error.response.status === 504) {
                console.warn(`[ScrapUrl] Python API reported timeout/block for ${url}`);
            } else {
                console.error(`[ScrapUrl] Error calling Python API for ${url}:`, error.message);
            }
            throw new Error(`ScrapUrl API failed: ${error.message}`);
        }
    });
}

module.exports = ScrapUrl;
