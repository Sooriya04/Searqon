/**
 * Classifier Service
 * Thin HTTP client that calls the Python Intelligence microservice (port 3003).
 * Endpoints consumed:
 *   POST /classify   → query routing (semantic intent)
 *   POST /summarize  → TF-IDF extractive highlights
 *
 * Used by: controller/classifierController.js, controller/unifiedController.js
 */

const http = require("http");

const CLASSIFIER_PORT = 3003;
const CLASSIFIER_TIMEOUT_MS = 5000; // 5s — pure math, no LLM needed

// ─── Generic HTTP Helper ──────────────────────────────────────────────────────

function postToClassifier(path, payload) {
    return new Promise((resolve, reject) => {
        const body = JSON.stringify(payload);

        const req = http.request(
            {
                hostname: "localhost",
                port: CLASSIFIER_PORT,
                path,
                method: "POST",
                headers: {
                    "Content-Type": "application/json",
                    "Content-Length": Buffer.byteLength(body),
                },
                timeout: CLASSIFIER_TIMEOUT_MS,
            },
            (res) => {
                let data = "";
                res.on("data", (chunk) => (data += chunk));
                res.on("end", () => {
                    try {
                        resolve(JSON.parse(data));
                    } catch (e) {
                        reject(new Error(`Invalid JSON from classifier ${path}`));
                    }
                });
            }
        );

        req.on("error", reject);
        req.on("timeout", () => {
            req.destroy();
            reject(new Error(`Classifier ${path} timeout`));
        });

        req.write(body);
        req.end();
    });
}

// ─── Route Query ──────────────────────────────────────────────────────────────

/**
 * Route a query to the relevant sources.
 * @param {string} query
 * @returns {Promise<{sources: string[], strategy: string, categories: string[]}>}
 */
async function routeQuery(query) {
    try {
        const result = await postToClassifier("/classify", { query });

        if (!result?.sources?.length) {
            throw new Error("Empty response from classifier");
        }

        console.log(
            `[Router] "${query}" → [${result.sources.join(", ")}] (${result.strategy})`
        );

        return {
            sources:    result.sources,
            strategy:   result.strategy,
            categories: result.categories || [],
        };
    } catch (err) {
        console.warn(`[Router] Classifier unavailable (${err.message}), using safe defaults`);
        return {
            sources:    ["wikipedia", "web", "duckduckgo"],
            strategy:   "emergency_fallback",
            categories: [],
        };
    }
}

// ─── Summarize Results ────────────────────────────────────────────────────────

/**
 * Send scraped documents to the Python TF-IDF summarizer.
 * @param {string} query - The original user query
 * @param {Array} documents - Array of { source, title, content, url }
 * @param {number} numHighlights - How many highlight sentences to extract
 * @returns {Promise<Array>} - Array of highlight objects
 */
async function summarizeResults(query, documents, numHighlights = 5) {
    try {
        const result = await postToClassifier("/summarize", {
            query,
            documents,
            num_highlights: numHighlights,
        });

        if (!result?.highlights) {
            throw new Error("No highlights in summarizer response");
        }

        console.log(`[Summarizer] Extracted ${result.highlights.length} highlights`);
        return result.highlights;
    } catch (err) {
        console.warn(`[Summarizer] Failed (${err.message}), skipping highlights`);
        return [];
    }
}

module.exports = { routeQuery, summarizeResults };
