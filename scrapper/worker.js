const { Worker: ThreadWorker } = require('worker_threads');
const path = require("path");
const os = require("os");

// ── Thread Pool ───────────────────────────────────────────────────
// Pre-warm a pool of parser threads to avoid spawn overhead per request.

const POOL_SIZE = Math.max(2, Math.min(os.cpus().length - 1, 8));
const pool = [];
const taskQueue = [];

function createThread() {
    const worker = new ThreadWorker(
        path.resolve(__dirname, 'parserThread.js')
    );

    const thread = { worker, busy: false };

    worker.on('message', (msg) => {
        const task = thread.currentTask;
        thread.busy = false;
        thread.currentTask = null;

        if (task) {
            if (msg.success) task.resolve(msg.result);
            else task.reject(new Error(msg.error));
        }

        // Process next queued task
        processQueue();
    });

    worker.on('error', (err) => {
        const task = thread.currentTask;
        thread.busy = false;
        thread.currentTask = null;

        if (task) task.reject(err);

        // Replace dead thread
        const idx = pool.indexOf(thread);
        if (idx !== -1) {
            pool[idx] = createThread();
        }

        processQueue();
    });

    return thread;
}

function processQueue() {
    if (taskQueue.length === 0) return;

    const freeThread = pool.find(t => !t.busy);
    if (!freeThread) return;

    const task = taskQueue.shift();
    freeThread.busy = true;
    freeThread.currentTask = task;
    freeThread.worker.postMessage({ html: task.html, url: task.url });
}

// Initialize pool
for (let i = 0; i < POOL_SIZE; i++) {
    pool.push(createThread());
}

console.log(`[ParserPool] Initialized ${POOL_SIZE} parser threads`);

/**
 * Parse HTML in a worker thread (uses thread pool)
 * @param {string} html - Raw HTML content
 * @param {string} url - Source URL (for resolving relative links)
 * @param {number} timeout - Timeout in ms (default 10000)
 * @returns {Promise<{title: string|null, content: string|null}>}
 */
function parseInThread(html, url, timeout = 10000) {
    return new Promise((resolve, reject) => {
        const timer = setTimeout(() => {
            reject(new Error(`Parse timeout after ${timeout}ms for ${url}`));
        }, timeout);

        const task = {
            html,
            url,
            resolve: (result) => { clearTimeout(timer); resolve(result); },
            reject: (err) => { clearTimeout(timer); reject(err); },
        };

        taskQueue.push(task);
        processQueue();
    });
}

module.exports = { parseInThread };
