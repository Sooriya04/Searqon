const puppeteer = require('puppeteer-extra');
const StealthPlugin = require('puppeteer-extra-plugin-stealth');

puppeteer.use(StealthPlugin());

let browserInstance = null;

async function getBrowser() {
    if (!browserInstance || !browserInstance.isConnected()) {
        console.log('[Browser] Launching new browser instance...');
        browserInstance = await puppeteer.launch({
            headless: "new",
            args: [
                '--no-sandbox',
                '--disable-setuid-sandbox',
                '--disable-dev-shm-usage',
                '--disable-accelerated-2d-canvas',
                '--no-first-run',
                '--no-zygote',
                // '--single-process', // Often causes issues in some envs, use with caution
                '--disable-gpu',
                '--window-size=1920,1080'
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
