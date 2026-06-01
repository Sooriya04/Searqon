const { getPool } = require('../config/database');

async function saveSearchResult({ query, source, title, url, content, score, wordCount, metadata }) {
  const pool = getPool();
  if (!pool) return null;

  try {
    const queryText = `
      INSERT INTO search_results (query, source, title, url, content, score, word_count, metadata, updated_at)
      VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW())
      ON CONFLICT (url) DO UPDATE
      SET query = EXCLUDED.query,
          title = EXCLUDED.title,
          content = EXCLUDED.content,
          score = EXCLUDED.score,
          word_count = EXCLUDED.word_count,
          metadata = EXCLUDED.metadata,
          updated_at = NOW()
      RETURNING *;
    `;
    const values = [query, source, title, url, content, score, wordCount, JSON.stringify(metadata || {})];
    const res = await pool.query(queryText, values);
    return res.rows[0];
  } catch (err) {
    console.error('[Database] Failed to save search result:', err.message);
    return null;
  }
}

async function getCachedSearchResults(query) {
  const pool = getPool();
  if (!pool) return null;

  try {
    const queryText = `
      SELECT * FROM search_results 
      WHERE query = $1 
      ORDER BY score DESC;
    `;
    const res = await pool.query(queryText, [query]);
    return res.rows.map(row => ({
      query: row.query,
      source: row.source,
      title: row.title,
      url: row.url,
      content: row.content,
      score: row.score,
      wordCount: row.word_count,
      metadata: row.metadata,
      createdAt: row.created_at,
      updatedAt: row.updated_at
    }));
  } catch (err) {
    console.error('[Database] Failed to fetch cached search results:', err.message);
    return null;
  }
}

module.exports = {
  saveSearchResult,
  getCachedSearchResults
};
