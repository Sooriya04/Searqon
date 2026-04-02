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
    // Try JSON code block first
    const block = raw.match(/```(?:json)?\s*([\s\S]*?)```/);
    if (block) {
        try { return JSON.parse(block[1].trim()); } catch (_) {}
    }

    // Try bare JSON object/array
    const obj = raw.match(/(\{[\s\S]*\}|\[[\s\S]*\])/);
    if (obj) {
        try { return JSON.parse(obj[1].trim()); } catch (_) {}
    }

    // Last resort: raw string
    try { return JSON.parse(raw.trim()); } catch (_) {}
    return raw.trim();
}

module.exports = { extract };
