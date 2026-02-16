const { parentPort } = require("worker_threads");
const { Readability } = require("@mozilla/readability");
const { parseHTML } = require("linkedom");

// Keep thread alive across multiple parse jobs
parentPort.on('message', ({ html, url }) => {
    try {
        const { document } = parseHTML(html);

        // linkedom doesn't support baseURI assignment — set it via <base> tag
        const base = document.createElement('base');
        base.setAttribute('href', url);
        document.head.appendChild(base);

        const reader = new Readability(document);
        const parsed = reader.parse();

        parentPort.postMessage({
            success: true,
            result: {
                title: parsed?.title || null,
                content: parsed?.textContent || parsed?.content || null,
            }
        });
    } catch (e) {
        parentPort.postMessage({
            success: false,
            error: e.message
        });
    }
});