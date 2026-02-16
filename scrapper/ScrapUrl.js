const { fetchUrl } = require("./fetcher");
const { parseInThread } = require("./worker");

async function ScrapUrl(url, options = {}) {
    const { fetchTimeout = 15000, parseTimeout = 10000 } = options;

    if (!url || typeof url !== 'string') {
        throw new Error('URL is required and must be a string');
    }

    const start = Date.now();

    // Fetch HTML using undici connection pool
    const html = await fetchUrl(url, fetchTimeout);

    if (!html || html.trim().length === 0) {
        throw new Error(`Empty response from ${url}`);
    }

    // Parse in worker thread (offloaded from event loop)
    const parsed = await parseInThread(html, url, parseTimeout);

    const content = parsed?.content || '';
    const wordCount = content.split(/\s+/).filter(w => w.length > 0).length;

    console.log(`[ScrapUrl] ${url} → ${wordCount} words in ${Date.now() - start}ms`);

    return {
        title: parsed?.title || '',
        content: content,
        url: url,
        wordCount: wordCount,
    };
}

module.exports = ScrapUrl;
