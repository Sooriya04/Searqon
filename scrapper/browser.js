const puppeteer = require('puppeteer-extra');
const StealthPlugin = require('puppeteer-extra-plugin-stealth');
const config = require('../utils/configLoader');

puppeteer.use(StealthPlugin());

let browserInstance = null;

async function getBrowser() {
    if (!browserInstance || !browserInstance.isConnected()) {
        console.log('[Browser] Launching new browser instance...');
        browserInstance = await puppeteer.launch({
            headless: config.browser.headless,
            args: [
                '--no-sandbox',
                '--disable-setuid-sandbox',
                '--disable-dev-shm-usage',
                '--disable-accelerated-2d-canvas',
                '--no-first-run',
                '--no-zygote',
                '--disable-gpu',
                `--window-size=${config.browser.window_size}`
            ]
        });

        browserInstance.on('disconnected', () => {
            console.warn('[Browser] Disconnected. Clearing instance.');
            browserInstance = null;
        });
    }
    return browserInstance;
}

module.exports = { getBrowser };
