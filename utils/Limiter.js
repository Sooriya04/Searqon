class Limiter {
    /**
     * @param {number} concurrency - Max active concurrent jobs
     * @param {number} queueLimit - Max jobs in queue before rejecting
     */
    constructor(concurrency = 3, queueLimit = 10) {
        this.concurrency = concurrency;
        this.queueLimit = queueLimit;
        this.active = 0;
        this.queue = [];
    }

    /**
     * Execute a function with concurrency control
     * @param {Function} fn - Async function to execute
     * @returns {Promise<any>}
     */
    async add(fn) {
        if (this.active >= this.concurrency) {
            if (this.queue.length >= this.queueLimit) {
                throw new Error('Service overloaded: Queue and concurrency limit reached');
            }
            console.log(`[Limiter] Queued (Active: ${this.active}, Queue: ${this.queue.length + 1})`);
            await new Promise((resolve, reject) => {
                this.queue.push({ resolve, reject });
            });
        }

        this.active++;
        try {
            return await fn();
        } finally {
            this.active--;
            if (this.queue.length > 0) {
                const next = this.queue.shift();
                next.resolve();
            }
        }
    }
}

// Default to safe limits for a single instance
module.exports = new Limiter(
    parseInt(process.env.MAX_CONCURRENT_SESSIONS || 3, 10),
    parseInt(process.env.QUEUE_LENGTH || 15, 10)
);
