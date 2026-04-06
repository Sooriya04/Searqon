/**
 * Searqon Extraction Service
 *
 * LLM-based structured data extraction for /api/extract.
 * Uses native fetch (Node 18+) — no axios.
 *
 * Backend is controlled by the EXTRACTION_BACKEND env var:
 *   EXTRACTION_BACKEND=ollama  → http://localhost:11434 (default, free)
 *   EXTRACTION_BACKEND=gemini  → requires GEMINI_API_KEY
 *   EXTRACTION_BACKEND=openai  → requires OPENAI_API_KEY
 */

const BACKEND      = process.env.EXTRACTION_BACKEND  || 'ollama';
const OLLAMA_URL   = process.env.OLLAMA_URL           || 'http://localhost:11434';
const OLLAMA_MODEL = process.env.OLLAMA_MODEL         || 'qwen2.5:0.5b';
const GEMINI_KEY   = process.env.GEMINI_API_KEY       || '';
const OPENAI_KEY   = process.env.OPENAI_API_KEY       || '';

// ─── Helper: JSON fetch with timeout ─────────────────────────────────────────

async function postJSON(url, body, timeoutMs = 60000) {
    const controller = new AbortController();
    const id = setTimeout(() => controller.abort(), timeoutMs);

    try {
        const res = await fetch(url, {
            method:  'POST',
            headers: { 'Content-Type': 'application/json' },
            body:    JSON.stringify(body),
            signal:  controller.signal
        });

        if (!res.ok) {
            const text = await res.text();
            throw new Error(`HTTP ${res.status}: ${text}`);
        }

        return await res.json();
    } finally {
        clearTimeout(id);
    }
}

// ─── Public API ───────────────────────────────────────────────────────────────

async function extract(prompt) {
    switch (BACKEND) {
        case 'gemini':  return await extractWithGemini(prompt);
        case 'openai':  return await extractWithOpenAI(prompt);
        case 'ollama':
        default:        return await extractWithOllama(prompt);
    }
}

/**
 * Synthesize a direct answer from multiple search results (RAG).
 * @param {string} query
 * @param {Array} documents - Array of { source, title, content, url }
 */
async function synthesizeAnswer(query, documents) {
    if (!documents || documents.length === 0) return null;

    // 1. Build context from top 3-5 documents
    const context = documents.slice(0, 5).map(d =>
        `Title: ${d.title}\nSource: ${d.url}\nContent: ${d.content.slice(0, 2000)}`
    ).join('\n\n---\n\n');

    // 2. Prepare the prompt
    const prompt = `You are Searqon-AI, a helpful and accurate search assistant. 
Your task is to provide a concise, direct, and factual answer to the query based ONLY on the provided context.

Query: "${query}"

Context from web:
${context}

Instructions:
- Be direct and start with the answer.
- If the information is not in the context, say "I couldn't find a definitive answer in the search results."
- Keep it under 3-4 sentences unless more detail is absolutely necessary.
- Use markdown for formatting (bolding important terms).
- If there are multiple viewpoints or rates, list them briefly.

Answer:`;

    // 3. Call current backend
    switch (BACKEND) {
        case 'gemini':  return await callGeminiRaw(prompt);
        case 'openai':  return await callOpenAIRaw(prompt);
        case 'ollama':
        default:        return await callOllamaRaw(prompt);
    }
}

// ─── Direct LLM Callers (Raw Text) ───────────────────────────────────────────

async function callOllamaRaw(prompt) {
    const data = await postJSON(`${OLLAMA_URL}/api/generate`, {
        model:   OLLAMA_MODEL,
        prompt:  prompt,
        stream:  false,
        options: { temperature: 0.1, num_predict: 512 }
    });
    return data?.response?.trim() || '';
}

async function callGeminiRaw(prompt) {
    if (!GEMINI_KEY) throw new Error('GEMINI_API_KEY is not set');
    const data = await postJSON(
        `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=${GEMINI_KEY}`,
        {
            contents: [{ parts: [{ text: prompt }] }],
            generationConfig: { temperature: 0.2, maxOutputTokens: 512 }
        }
    );
    return data?.candidates?.[0]?.content?.parts?.[0]?.text?.trim() || '';
}

async function callOpenAIRaw(prompt) {
    if (!OPENAI_KEY) throw new Error('OPENAI_API_KEY is not set');
    const res = await fetch('https://api.openai.com/v1/chat/completions', {
        method:  'POST',
        headers: {
            'Authorization': `Bearer ${OPENAI_KEY}`,
            'Content-Type':  'application/json'
        },
        body: JSON.stringify({
            model:       'gpt-4o-mini',
            messages:    [{ role: 'user', content: prompt }],
            temperature: 0.2,
            max_tokens:  512
        })
    });
    const data = await res.json();
    return data?.choices?.[0]?.message?.content?.trim() || '';
}

// ─── Ollama (local) ───────────────────────────────────────────────────────────

async function extractWithOllama(prompt) {
    const data = await postJSON(`${OLLAMA_URL}/api/generate`, {
        model:   OLLAMA_MODEL,
        prompt:  prompt,
        stream:  false,
        options: { temperature: 0.1, num_predict: 2048 }
    });

    return parseJSON(data?.response || '');
}

// ─── Gemini ───────────────────────────────────────────────────────────────────

async function extractWithGemini(prompt) {
    if (!GEMINI_KEY) throw new Error('GEMINI_API_KEY is not set');

    const data = await postJSON(
        `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.0-flash:generateContent?key=${GEMINI_KEY}`,
        {
            contents:         [{ parts: [{ text: prompt }] }],
            generationConfig: { temperature: 0.1, maxOutputTokens: 2048 }
        }
    );

    const raw = data?.candidates?.[0]?.content?.parts?.[0]?.text || '';
    return parseJSON(raw);
}

// ─── OpenAI ───────────────────────────────────────────────────────────────────

async function extractWithOpenAI(prompt) {
    if (!OPENAI_KEY) throw new Error('OPENAI_API_KEY is not set');

    const controller = new AbortController();
    const id = setTimeout(() => controller.abort(), 60000);

    try {
        const res = await fetch('https://api.openai.com/v1/chat/completions', {
            method:  'POST',
            headers: {
                'Authorization': `Bearer ${OPENAI_KEY}`,
                'Content-Type':  'application/json'
            },
            body: JSON.stringify({
                model:       'gpt-4o-mini',
                messages:    [
                    { role: 'system', content: 'You are a structured data extraction assistant. Always respond with valid JSON.' },
                    { role: 'user', content: prompt }
                ],
                temperature: 0.1,
                max_tokens:  2048
            }),
            signal: controller.signal
        });
        clearTimeout(id);

        if (!res.ok) throw new Error(`OpenAI HTTP ${res.status}`);

        const data = await res.json();
        const raw  = data?.choices?.[0]?.message?.content || '';
        return parseJSON(raw);
    } finally {
        clearTimeout(id);
    }
}

// ─── JSON Parser ──────────────────────────────────────────────────────────────

function parseJSON(raw) {
    if (!raw) return null;

    let jsonStr = raw.trim();

    // 1. Try to extract from markdown code blocks
    const blockMatch = jsonStr.match(/```(?:json)?\s*([\s\S]*?)```/);
    if (blockMatch) {
        jsonStr = blockMatch[1].trim();
    } else {
        // 2. Try to find the first '{' or '[' and the last '}' or ']'
        const firstBrace = jsonStr.indexOf('{');
        const firstBracket = jsonStr.indexOf('[');
        let start = -1;
        let end = -1;

        if (firstBrace !== -1 && (firstBracket === -1 || firstBrace < firstBracket)) {
            start = firstBrace;
            end = jsonStr.lastIndexOf('}');
        } else if (firstBracket !== -1) {
            start = firstBracket;
            end = jsonStr.lastIndexOf(']');
        }

        if (start !== -1 && end !== -1 && end > start) {
            jsonStr = jsonStr.substring(start, end + 1);
        }
    }

    // 3. Attempt initial parse
    try {
        return JSON.parse(jsonStr);
    } catch (err) {
        // 4. Heuristic cleanup for common LLM sloppy JSON
        try {
            // Remove thousands separators (e.g., 18,000,000 -> 18000000)
            // Only targets commas between digits that look like thousands (3 digits following)
            let fixed = jsonStr.replace(/(\d),(\d{3}(?!\d))/g, '$1$2');

            // Fix trailing commas in objects or arrays (e.g., [1, 2,] -> [1, 2])
            fixed = fixed.replace(/,\s*([\]}])/g, '$1');

            return JSON.parse(fixed);
        } catch (err2) {
            console.warn('[Extraction] Failed to parse JSON even after cleanup heuristics.');
            // Fallback: if it starts with { or [, it's probably intended to be JSON but too broken
            // otherwise it might be just a text response.
            if (jsonStr.startsWith('{') || jsonStr.startsWith('[')) {
                return { error: 'Invalid JSON format from LLM', raw: jsonStr.slice(0, 500) };
            }
            return jsonStr;
        }
    }
}

module.exports = { extract, synthesizeAnswer, BACKEND };
