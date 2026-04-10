const httpClient = require('../utils/httpClient');
const axios = require('axios');
const limiter = require("../utils/Limiter");
const config = require('../utils/configLoader');
const { BROWSER_HEADERS } = require('../utils/browserHeaders');
const { cleanYouTubeDescription } = require('../utils/textCleaner');

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
                let cleanReadme = readmeRes.data
                    .replace(/```[\s\S]*?```/g, '')
                    .replace(/`([^`]+)`/g, '$1')
                    .replace(/^#+\s/gm, '')
                    .replace(/[┌┬┐├┼┤└┴┘│─━┏┳┓┣╋┫┗┻┛┃]/g, '')
                    .replace(/\[([^\]]+)\]\([^\)]+\)/g, '$1')
                    .replace(/\n/g, ' ')
                    .replace(/\*/g, '')
                    .replace(/\s{2,}/g, ' ')
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

    // YouTube: extract description from internal metadata (avoiding footer junk)
    const youtubeMatch = url.match(/^https?:\/\/(?:www\.)?(?:youtube\.com\/watch\?v=|youtu\.be\/)([a-zA-Z0-9_-]{11})/);
    if (youtubeMatch) {
        try {
            console.error(`[ScrapUrl] YouTube intercept -> ${url}`);
            const ytResponse = await httpClient.get(url, {
                headers: { ...BROWSER_HEADERS, 'Accept-Language': 'en-US' },
                timeout: 15000
            });
            const html = ytResponse.data;
            
            // Log if we hitting a consent wall
            if (html.includes('consent.youtube.com') || html.includes('Before you continue to YouTube')) {
                console.warn(`[ScrapUrl] Hit YouTube consent wall for ${url}`);
            }

            let description = "";
            let title = "YouTube Video";

            // 1. Try ytInitialPlayerResponse
            const playerResponseMatch = html.match(/ytInitialPlayerResponse\s*=\s*({[\s\S]*?});/);
            if (playerResponseMatch) {
                try {
                    const data = JSON.parse(playerResponseMatch[1]);
                    title = data.videoDetails?.title || title;
                    description = data.videoDetails?.shortDescription || "";
                    if (description) console.log(`[ScrapUrl] Got description from ytInitialPlayerResponse (${description.length} chars)`);
                } catch (e) {
                    console.warn(`[ScrapUrl] JSON.parse(ytInitialPlayerResponse) failed`);
                }
            }

            // 2. Try ytInitialData (secondary extraction)
            if (!description) {
                const initialDataMatch = html.match(/ytInitialData\s*=\s*({[\s\S]*?});/);
                if (initialDataMatch) {
                    try {
                        const data = JSON.parse(initialDataMatch[1]);
                        const secondary = data.contents?.twoColumnWatchNextResults?.results?.results?.contents;
                        if (secondary) {
                            const videoSecondaryInfo = secondary.find(c => c.videoSecondaryInfoRenderer)?.videoSecondaryInfoRenderer;
                            if (videoSecondaryInfo?.description) {
                                description = videoSecondaryInfo.description.runs?.map(r => r.text).join('') || videoSecondaryInfo.description.simpleText || "";
                                if (description) console.log(`[ScrapUrl] Got description from ytInitialData (${description.length} chars)`);
                            }
                        }
                    } catch (e) { }
                }
            }

            // 3. Raw search for "shortDescription" if JSON parsing was flaky
            if (!description) {
                const rawDescMatch = html.match(/"shortDescription":"([\s\S]*?)(?<!\\)"/);
                if (rawDescMatch) {
                    description = rawDescMatch[1]
                        .replace(/\\n/g, '\n')
                        .replace(/\\"/g, '"')
                        .replace(/\\u([0-9a-fA-F]{4})/g, (match, grp) => String.fromCharCode(parseInt(grp, 16)));
                    if (description) console.log(`[ScrapUrl] Got description from raw regex search (${description.length} chars)`);
                }
            }

            // 4. Meta tag fallback (usually truncated but better than nothing)
            if (!description) {
                const metaMatch = html.match(/<meta\s+name="description"\s+content="([^"]*)"/i) || html.match(/<meta\s+property="og:description"\s+content="([^"]*)"/i);
                if (metaMatch) {
                    description = metaMatch[1];
                    console.log(`[ScrapUrl] Falling back to meta description tag`);
                }
            }

            if (description) {
                console.error(`[ScrapUrl] SUCCESS: Extracted ${description.length} chars for ${url}`);
                return {
                    title: title,
                    content: description,
                    description: description,
                    url: url,
                    wordCount: description.split(/\s+/).length,
                    duration: 500
                };
            } else {
                console.error(`[ScrapUrl] FAILURE: No description found for ${url}`);
            }
        } catch (e) {
            console.error(`[ScrapUrl] YouTube bypass error: ${e.message}`);
        }
    }

    return limiter.add(async () => {
        const startTime = Date.now();
        const timeout = options.timeout || config.browser.timeout || 10000;
        const format = options.format || 'markdown';

        try {
            console.log(`[ScrapUrl] Go scraper -> ${url} (format: ${format})`);
            const response = await axios.post(`${GO_SCRAPER_URL}/scrape`, { url, format }, {
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
    const format = options.format || 'markdown';

    // Separate bypass URLs (GitHub, YouTube) from normal ones
    const bypassPatterns = [
        /^https?:\/\/(?:www\.)?github\.com\/[^\/]+\/[^\/#?]+/,
        /^https?:\/\/(?:www\.)?(?:youtube\.com\/watch\?v=|youtu\.be\/)[a-zA-Z0-9_-]{11}/
    ];

    const bypassUrls = [];
    const normalUrls = [];

    validUrls.forEach(u => {
        if (bypassPatterns.some(p => p.test(u))) {
            bypassUrls.push(u);
        } else {
            normalUrls.push(u);
        }
    });

    try {
        const resultsMap = {};

        // 1. Process bypass URLs in parallel (utilizes optimized Node.js logic)
        if (bypassUrls.length > 0) {
            console.log(`[ScrapUrl Batch] Processing ${bypassUrls.length} bypass URLs (Node.js)...`);
            const bypassResults = await Promise.all(
                bypassUrls.map(u => ScrapUrl(u, options).catch(e => ({
                    title: '', content: '', url: u, wordCount: 0, error: e.message
                })))
            );
            bypassResults.forEach(r => resultsMap[r.url] = r);
        }

        // 2. Process remaining URLs via Go Batch Scraper
        if (normalUrls.length > 0) {
            console.log(`[ScrapUrl Batch] Sending ${normalUrls.length} normal URLs to Go scraper (format: ${format})...`);
            const response = await axios.post(`${GO_SCRAPER_URL}/scrape/batch`, { urls: normalUrls, format }, {
                timeout: timeout * 2,
                headers: { 'Content-Type': 'application/json' }
            });
            response.data.forEach(r => resultsMap[r.url] = r);
        }

        // Return results in original order
        return validUrls.map(u => resultsMap[u]);

    } catch (error) {
        console.error(`[ScrapUrl Batch] Failed, falling back to individual scraping:`, error.message);
        return Promise.all(validUrls.map(u => ScrapUrl(u, options).catch(() => ({
            title: '', content: '', url: u, wordCount: 0, error: 'failed'
        }))));
    }
}

module.exports = ScrapUrl;
module.exports.ScrapUrlBatch = ScrapUrlBatch;
