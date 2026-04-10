/**
 * Classifier Service
 * Consolidates the Intelligence Layer into native JavaScript.
 */

const { classifyQuery, tfidfSummarize } = require("./intelligence");

/**
 * Route a query to the relevant sources using native JS logic.
 * @param {string} query
 * @returns {Promise<{sources: string[], strategy: string, categories: string[]}>}
 */
async function routeQuery(query) {
    try {
        const result = classifyQuery(query);

        console.log(
            `[Router] "${query}" → [${result.sources.join(", ")}] (${result.strategy})`
        );

        return {
            sources:    result.sources,
            strategy:   result.strategy,
            categories: result.categories || [],
        };
    } catch (err) {
        console.warn(`[Router] Classifier failed (${err.message}), using safe defaults`);
        return {
            sources:    ["wikipedia", "web", "duckduckgo"],
            strategy:   "emergency_fallback",
            categories: [],
        };
    }
}

/**
 * Send scraped documents to the native JS TF-IDF summarizer.
 * @param {string} query - The original user query
 * @param {Array} documents - Array of { source, title, content, url }
 * @param {number} numHighlights - How many highlight sentences to extract
 * @returns {Promise<Array>} - Array of highlight objects
 */
async function summarizeResults(query, documents, numHighlights = 5) {
    try {
        const highlights = tfidfSummarize(query, documents, numHighlights);

        console.log(`[Summarizer] Extracted ${highlights.length} highlights`);
        return highlights;
    } catch (err) {
        console.warn(`[Summarizer] Failed (${err.message}), skipping highlights`);
        return [];
    }
}

module.exports = { routeQuery, summarizeResults };
