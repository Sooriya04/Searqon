const axios = require('axios');
const limiter = require("../utils/Limiter");
const config = require('../utils/configLoader');

const GO_SCRAPER_URL = 'http://127.0.0.1:3002';
const BINARY_EXTENSIONS = /\.(jpg|jpeg|png|gif|pdf|zip|mp4|webp|svg)$/i;

async function ScrapUrl(url, options = {}) {
    if (BINARY_EXTENSIONS.test(url)) {
        console.warn(`[ScrapUrl] Skipping binary/image URL: ${url}`);
        return { title: "Binary File", content: "", url: url, wordCount: 0, duration: 0 };
    }

    // GitHub: fetch README directly via API (fastest path, no scraping needed)
    const githubMatch = url.match(/^https?:\/\/(?:www\.)?github\.com\/([^\/]+)\/([^\/#?]+)/);
    if (githubMatch) {
        const owner = githubMatch[1];
        const repo = githubMatch[2];
        try {
            console.log(`[ScrapUrl] GitHub intercept, fetching README: ${url}`);
            const readmeRes = await axios.get(`https://api.github.com/repos/${owner}/${repo}/readme`, {
                headers: { 'Accept': 'application/vnd.github.v3.raw', 'User-Agent': 'SearqonBot/1.0' },
                timeout: 10000
            });
            if (readmeRes.data && typeof readmeRes.data === 'string') {
                // Strip markdown and ASCII box drawing chars for clean plain text
                let cleanReadme = readmeRes.data
                    .replace(/```[\s\S]*?```/g, '') // remove large code blocks
                    .replace(/`([^`]+)`/g, '$1') // remove inline code backticks
                    .replace(/^#+\s/gm, '') // remove heading hashes
                    .replace(/[┌┬┐├┼┤└┴┘│─━┏┳┓┣╋┫┗┻┛┃]/g, '') // remove ascii box drawing
                    .replace(/\[([^\]]+)\]\([^\)]+\)/g, '$1') // replace links with just text
                    .replace(/\n/g, ' ') // replace all newlines with spaces
                    .replace(/\*/g, '') // remove all asterisks
                    .replace(/\s{2,}/g, ' ') // collapse multiple spaces
                    .trim();
                
                return {
                    title: `${repo} by ${owner} - GitHub`,
                    content: cleanReadme,
                    url: url,
                    wordCount: cleanReadme.split(/\s+/).length,
                    duration: 200
                };
            }
        } catch (e) {
            console.warn(`[ScrapUrl] GitHub README fetch failed: ${e.message}`);
        }
    }

    return limiter.add(async () => {
        const startTime = Date.now();
        const timeout = options.timeout || config.browser.timeout || 15000;

        try {
            console.log(`[ScrapUrl] Go scraper -> ${url}`);
            const response = await axios.post(`${GO_SCRAPER_URL}/scrape`, { url }, {
                timeout: timeout,
                headers: { 'Content-Type': 'application/json' }
            });

            const report = response.data;
            console.log(`[ScrapUrl] Done: ${url} (${report.wordCount} words, ${report.duration}ms)`);
            return report;
        } catch (error) {
            if (error.response && error.response.status === 504) {
                console.warn(`[ScrapUrl] Go scraper timeout for ${url}`);
            } else {
                console.error(`[ScrapUrl] Go scraper error for ${url}:`, error.message);
            }
            throw new Error(`ScrapUrl failed: ${error.message}`);
        }
    });
}

/**
 * Scrape multiple URLs in parallel using the Go batch endpoint.
 * This is MUCH faster than calling ScrapUrl() in a loop because
 * Go handles all the goroutines internally.
 */
async function ScrapUrlBatch(urls, options = {}) {
    // Filter out binary URLs
    const validUrls = urls.filter(u => !BINARY_EXTENSIONS.test(u));
    if (validUrls.length === 0) return [];

    const timeout = options.timeout || config.browser.timeout || 15000;

    try {
        console.log(`[ScrapUrl Batch] Sending ${validUrls.length} URLs to Go scraper...`);
        const response = await axios.post(`${GO_SCRAPER_URL}/scrape/batch`, { urls: validUrls }, {
            timeout: timeout * 2, // Batch needs more time
            headers: { 'Content-Type': 'application/json' }
        });

        const results = response.data;
        console.log(`[ScrapUrl Batch] Got ${results.length} results`);
        return results;
    } catch (error) {
        console.error(`[ScrapUrl Batch] Failed:`, error.message);
        // Fallback: try individual scraping
        return Promise.all(validUrls.map(u => ScrapUrl(u, options).catch(() => ({
            title: '', content: '', url: u, wordCount: 0, error: 'failed'
        }))));
    }
}

module.exports = ScrapUrl;
module.exports.ScrapUrlBatch = ScrapUrlBatch;
