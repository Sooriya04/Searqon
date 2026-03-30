/**
 * Classifier Service
 * Thin HTTP client that calls the Python classifier microservice (port 3003).
 * All routing logic (LLM via qwen2.5:0.5b + keyword fallback) lives in classifier/classifier.py.
 * Used by: controller/classifierController.js, controller/unifiedController.js
 */

const http = require("http");

const CLASSIFIER_TIMEOUT_MS = 30000; // 30s — Ollama inference needs time

/**
 * Call the Python classifier microservice.
 * @param {string} query
 * @returns {Promise<{sources: string[], strategy: string, categories: string[]}>}
 */
function callClassifier(query) {
    return new Promise((resolve, reject) => {
        const body = JSON.stringify({ query });

        const req = http.request(
            {
                hostname: "localhost",
                port: 3003,
                path: "/classify",
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
                        reject(new Error("Invalid JSON from classifier"));
                    }
                });
            }
        );

        req.on("error", reject);
        req.on("timeout", () => {
            req.destroy();
            reject(new Error("Classifier timeout"));
        });

        req.write(body);
        req.end();
    });
}

/**
 * Route a query to the relevant sources.
 * Returns an object with: sources[], strategy, categories[]
 *
 * DuckDuckGo is always the last source (guaranteed by the Python classifier).
 *
 * @param {string} query
 * @returns {Promise<{sources: string[], strategy: string, categories: string[]}>}
 */
async function routeQuery(query) {
    try {
        const result = await callClassifier(query);

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
        // Classifier is down or errored — should not happen as Python has its own fallback,
        // but this is a last-resort safety net.
        console.warn(`[Router] Classifier unavailable (${err.message}), using safe defaults`);
        return {
            sources:    ["wikipedia", "web", "duckduckgo"],
            strategy:   "emergency_fallback",
            categories: [],
        };
    }
}

module.exports = { routeQuery };
