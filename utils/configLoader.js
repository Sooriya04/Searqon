const fs = require('fs');
const path = require('path');
const yaml = require('js-yaml');

const SETTINGS_PATH = path.join(__dirname, '../settings.yaml');

let config = null;

function loadConfig() {
    if (config) return config;

    try {
        const fileContents = fs.readFileSync(SETTINGS_PATH, 'utf8');
        config = yaml.load(fileContents);
        console.log('[Config] Settings loaded successfully from settings.yaml');
    } catch (e) {
        console.error('[Config] Failed to load settings.yaml, using defaults:', e.message);
        config = {
            browser: {
                headless: "new",
                window_size: "1920,1080",
                timeout: 15000
            },
            concurrency: {
                max_active_sessions: 3,
                queue_limit: 15
            },
            scraping: {
                block_resources: ["image", "stylesheet", "font", "media"],
                user_agent: "Searqon/1.0"
            }
        };
    }
    return config;
}

module.exports = loadConfig();
