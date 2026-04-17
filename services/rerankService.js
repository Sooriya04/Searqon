const config = require('../utils/configLoader');
const { nodeRerank } = require('../utils/nodeReranker');

/**
 * Routes the reranking request to either the Python NLP engine or the local Node.js engine.
 * 
 * @param {string} query - The search query
 * @param {Array} documents - List of documents with title, url, content, source
 * @param {number} limit - How many top results to return
 * @returns {Promise<Array>} - The ranked results
 */
async function rerankResults(query, documents, limit = 10) {
    if (!documents || documents.length === 0) return [];
    
    const usePython = config.providers?.python_rerank === true;
    const useNode   = config.providers?.node_rerank !== false; // Default to true if not explicitly off

    // 1. Python NLP Strategy (Deep Meaning, High RAM)
    if (usePython) {
        const RERANK_URL = process.env.RERANK_URL || 'http://127.0.0.1:8001/rerank';
        try {
            const response = await fetch(RERANK_URL, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    query,
                    documents: documents.map(d => ({
                        title: d.title || "No Title",
                        url: d.url || "",
                        content: d.content || d.snippet || d.summary || "",
                        source: d.source || "unknown"
                    })),
                    limit: parseInt(limit, 10)
                }),
                signal: AbortSignal.timeout(5000) // 5s timeout for AI response
            });

            if (response.ok) {
                const data = await response.json();
                return data;
            }
            console.error(`[Rerank] Python engine error: ${response.status}`);
        } catch (err) {
            console.warn(`[Rerank] Python engine offline, falling back to local mode.`);
        }
    }

    // 2. Node.js Strategy (Fast, Ultra-Lightweight)
    if (useNode) {
        return nodeRerank(query, documents, limit);
    }

    // 3. Fallback (Raw)
    return documents.slice(0, limit);
}

module.exports = {
    rerankResults
};
