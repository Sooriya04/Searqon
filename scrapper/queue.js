// Handles distributed job scheduling

const { Queue } = require("bullmq");
const IORedis = require("ioredis");

const connection = new IORedis({
    host: process.env.REDIS_HOST || "127.0.0.1",
    port: parseInt(process.env.REDIS_PORT, 10) || 6379,
    maxRetriesPerRequest: null,
});

connection.on('error', (err) => {
    console.error('[Redis] Connection error:', err.message);
});

// creates BullMQ queue
const scrapeQueue = new Queue('scrapeQueue', {
    connection: connection
});

// Redis is used internally by BullMQ
module.exports = {
    scrapeQueue, connection
};