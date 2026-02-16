const { Pool } = require("undici");

// Connection pool cache — reuse connections per origin
const pools = new Map();

function getPool(origin) {
    if (!pools.has(origin)) {
        pools.set(origin, new Pool(origin, {
            connections: 10,         // Max concurrent connections per host
            pipelining: 1,
            keepAliveTimeout: 30000, // 30s keep-alive
            keepAliveMaxTimeout: 60000,
        }));
    }
    return pools.get(origin);
}

/**
 * Fetch a URL using undici connection pooling
 * @param {string} url - URL to fetch
 * @param {number} timeout - Request timeout in ms (default 15000)
 * @returns {Promise<string>} HTML content
 */
async function fetchUrl(url, timeout = 15000) {
    const parsed = new URL(url);
    const pool = getPool(parsed.origin);

    const { body, statusCode } = await pool.request({
        path: parsed.pathname + parsed.search,
        method: "GET",
        headers: {
            'user-agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/121.0 Safari/537.36 SearqonBot/1.0',
            'accept': 'text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8',
            'accept-encoding': 'gzip, deflate, br',
            'accept-language': 'en-US,en;q=0.9',
        },
        maxRedirections: 5,
        headersTimeout: timeout,
        bodyTimeout: timeout,
    });

    if (statusCode < 200 || statusCode >= 300) {
        // Drain the body to avoid memory leaks
        await body.dump();
        throw new Error(`HTTP ${statusCode} for ${url}`);
    }

    return await body.text();
}

module.exports = { fetchUrl };