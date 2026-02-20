const { getBrowser } = require("./browser");
const limiter = require("../utils/Limiter");
const config = require('../utils/configLoader');

async function ScrapUrl(url, options = {}) {
    return limiter.add(async () => {
        const startTime = Date.now();
        console.log(`[ScrapUrl] Started: ${url} at ${new Date(startTime).toISOString()}`);

        const browser = await getBrowser();
        const page = await browser.newPage();

        try {
            await page.setRequestInterception(true);
            page.on('request', (req) => {
                if (config.scraping.block_resources.includes(req.resourceType())) {
                    req.abort();
                } else {
                    req.continue();
                }
            });

            const timeout = options.timeout || config.browser.timeout;

            // Navigate
            await page.goto(url, {
                waitUntil: 'domcontentloaded',
                timeout: timeout
            });

            const result = await page.evaluate(() => {
                const scripts = document.querySelectorAll('script, style, noscript, iframe, svg, nav');
                scripts.forEach(s => s.remove());

                const title = document.title;

                const body = document.body ? document.body.innerText : '';

                const cleanBody = body
                    .split('\n')
                    .map(line => line.trim())
                    .filter(line => line.length > 0)
                    .join('\n');

                return {
                    title: title.trim(),
                    content: cleanBody
                };
            });

            const wordCount = result.content ? result.content.split(/\s+/).length : 0;
            const endTime = Date.now();

            const report = {
                title: result.title,
                content: result.content,
                url: url,
                wordCount: wordCount,
                startTime: new Date(startTime).toISOString(),
                endTime: new Date(endTime).toISOString(),
                duration: endTime - startTime
            };

            console.log(`[ScrapUrl] Finished: ${url}`);
            console.log(`Title: ${result.title}`);
            console.log(`Started: ${report.startTime}`);
            console.log(`Ended:   ${report.endTime}`);
            console.log(`Duration: ${report.duration}ms`);

            return report;
        } catch (error) {
            // Ignore resource blocking errors (expected due to request interception)
            if (error.message.includes('net::ERR_BLOCKED_BY_CLIENT')) {
                console.warn(`[ScrapUrl] Resource blocked on ${url} (expected)`);
            } else {
                console.error(`[ScrapUrl] Error scraping ${url}:`, error.message);
            }
            throw error;
        } finally {
            if (page) await page.close();
        }
    });
}

module.exports = ScrapUrl;
