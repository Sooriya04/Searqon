const { Pool } = require('pg');

let pool = null;

const connectDB = async () => {
  const connectionString = process.env.DATABASE_URL || process.env.POSTGRES_URI;
  if (!connectionString) {
    console.warn("[Database] DATABASE_URL or POSTGRES_URI not found in environment. Database caching is disabled.");
    return null;
  }

  try {
    pool = new Pool({
      connectionString,
      ssl: process.env.DATABASE_SSL === 'true' ? { rejectUnauthorized: false } : false
    });

    // Test connection
    const client = await pool.connect();
    console.log("[Database] PostgreSQL connected successfully");
    
    // Create search_results table if not exists
    await client.query(`
      CREATE TABLE IF NOT EXISTS search_results (
        id SERIAL PRIMARY KEY,
        query TEXT NOT NULL,
        source TEXT NOT NULL,
        title TEXT NOT NULL,
        url TEXT UNIQUE NOT NULL,
        content TEXT NOT NULL,
        score DOUBLE PRECISION DEFAULT 0.5,
        word_count INTEGER,
        metadata JSONB,
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
      );
      CREATE INDEX IF NOT EXISTS idx_search_results_query ON search_results(query);
    `);
    client.release();
    return pool;
  } catch (err) {
    console.error("[Database] PostgreSQL connection failed:", err.message);
    console.warn("[Database] Running without database features.");
    pool = null;
    return null;
  }
};

const getPool = () => pool;

module.exports = { connectDB, getPool };
