const config = require('./configLoader');

class Limiter {
    constructor(concurrency = 10, queueLimit = 25) {
        this.concurrency = concurrency;
        this.queueLimit = queueLimit;
        this.active = 0;
        this.queue = [];
    }
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

// Default to settings from settings.yaml
module.exports = new Limiter(
    config.concurrency.max_active_sessions,
    config.concurrency.queue_limit
);
