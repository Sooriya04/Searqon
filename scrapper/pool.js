const { fork } = require('child_process');
const path = require('path');
const os = require('os');

class ScraperPool {
  constructor(maxConcurrency = os.cpus().length) {
    this.maxConcurrency = maxConcurrency;
    this.workers = [];
    this.queue = [];
    this.activeWorkers = 0;
    this.workerScript = path.join(__dirname, 'worker.js');
    this.taskIdCounter = 0;
    this.pendingTasks = new Map(); // Map<id, {resolve, reject, timer}>
  }

  getWorker() {
    // Basic implementation: Create a new worker if under limit, or recycle?
    // For stability, let's keep a set of persistent workers.

    if (this.workers.length < this.maxConcurrency) {
      const worker = fork(this.workerScript);

      worker.on('message', (msg) => {
        const { id, status, data, error } = msg;
        const task = this.pendingTasks.get(id);

        if (task) {
          // Clear timeout if any
          // clearTimeout(task.timer);

          if (status === 'success') {
            task.resolve(data);
          } else {
            task.reject(new Error(error));
          }
          this.pendingTasks.delete(id);
        }

        // Worker is free now
        this.activeWorkers--;
        this.processQueue(worker);
      });

      worker.on('exit', (code) => {
        // Remove from workers list if it exits unexpectedly
        this.workers = this.workers.filter((w) => w !== worker);
        if (code !== 0) {
          console.error(`Worker exited with code ${code}`);
        }
        // If we had pending tasks on this worker, we might need to handle that,
        // but for now we assume 1 task per worker at a time in this simple model,
        // effectively 'activeWorkers' tracks busy state.
        // A more robust pool tracks which worker has which task.
      });

      this.workers.push(worker);
      return worker;
    }

    // Find an idle worker (conceptually, we need to track if they are busy)
    // To simplify: we will require a worker to send a message back when done to be "free".
    // But we are using a queue.

    // Actually, simpler approach for pool:
    // Just round-robin or first available?
    // Let's attach a 'busy' flag to the worker object wrapper.

    return null;
  }

  // Refined approach:
  // We maintain a list of helper objects: { worker, busy }

  init() {
    for (let i = 0; i < this.maxConcurrency; i++) {
      const process = fork(this.workerScript);
      const workerObj = { process, busy: false, id: i };

      process.on('message', (msg) => {
        const { id, status, data, error } = msg;
        const task = this.pendingTasks.get(id);
        if (task) {
          if (status === 'success') {
            task.resolve(data);
          } else {
            task.reject(new Error(error));
          }
          this.pendingTasks.delete(id);
        }

        workerObj.busy = false;
        this.processNext(workerObj);
      });

      process.on('exit', () => {
        // Handle crash (restart logic could go here)
        console.error(`Worker ${workerObj.id} died.`);
        workerObj.dead = true;
        // Ideally replace it
      });

      this.workers.push(workerObj);
    }
  }

  processNext(workerObj) {
    if (this.queue.length > 0) {
      const nextTask = this.queue.shift();
      workerObj.busy = true;
      this.executeTask(workerObj, nextTask);
    }
  }

  executeTask(workerObj, task) {
    const { id, url, options } = task;
    workerObj.process.send({ id, url, options });
  }

  schedule(url, options = {}) {
    if (this.workers.length === 0) {
      this.init();
    }

    return new Promise((resolve, reject) => {
      const id = ++this.taskIdCounter;
      this.pendingTasks.set(id, { resolve, reject });

      const task = { id, url, options };

      // Find available worker
      const availableWorker = this.workers.find((w) => !w.busy && !w.dead);

      if (availableWorker) {
        availableWorker.busy = true;
        this.executeTask(availableWorker, task);
      } else {
        this.queue.push(task);
      }
    });
  }

  /**
   * Process a batch of URLs concurrently
   * @param {string[]} urls - Array of URLs to scrape
   * @param {object} options - Options passed to scraper
   * @returns {Promise<Array>} Array of results (or error objects)
   */
  async scrapeMultiple(urls, options = {}) {
    console.log(`[Pool] Scheduling batch of ${urls.length} URLs`);
    const promises = urls.map((url) =>
      this.schedule(url, options).catch((err) => ({ error: err.message, url })),
    );
    return Promise.all(promises);
  }
}

// Singleton instance
const pool = new ScraperPool(10); // Fixed at 10 workers as requested for speed
module.exports = pool;
