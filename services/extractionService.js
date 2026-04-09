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
    const topDocs = documents.slice(0, 5);
    const context = topDocs.map((d, i) =>
        `[${i + 1}] Title: ${d.title}\nSource: ${d.url}\nContent: ${d.content.slice(0, 2000)}`
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
- INLINE CITATIONS REQUIRED: For every factual claim you make, append the corresponding source number in brackets, e.g., [1], [2], or [1,3]. Use the exact numbering provided in the context blocks above.

Answer:`;

    // 3. Call current backend
    let answerText = '';
    switch (BACKEND) {
        case 'gemini':  answerText = await callGeminiRaw(prompt); break;
        case 'openai':  answerText = await callOpenAIRaw(prompt); break;
        case 'ollama':
        default:        answerText = await callOllamaRaw(prompt); break;
    }

    return {
        text: answerText,
        references: topDocs.map((d, i) => ({
            id: i + 1,
            title: d.title,
            url: d.url
        }))
    };
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

// ─── Stream LLM Callers (SSE Generators via Python Proxy) ────────────────────

const http = require("http");

function callPythonStream(prompt) {
    return new Promise((resolve, reject) => {
        const req = http.request(
            {
                hostname: "localhost",
                port: 3004,
                path: "/stream",
                method: "POST",
                headers: { "Content-Type": "application/json" }
            },
            (res) => {
                if (res.statusCode !== 200) {
                    return reject(new Error(`Python stream rejected: ${res.statusCode}`));
                }
                const { Transform } = require('stream');
                resolve(res);
            }
        );
        req.on("error", reject);
        req.write(JSON.stringify({ backend: BACKEND, prompt }));
        req.end();
    });
}

/**
 * Synthesize answer using streaming generator (proxied to Python)
 */
async function* synthesizeAnswerStream(query, documents) {
    if (!documents || documents.length === 0) return;

    const topDocs = documents.slice(0, 5);
    const context = topDocs.map((d, i) =>
        `[${i + 1}] Title: ${d.title}\nSource: ${d.url}\nContent: ${d.content.slice(0, 2000)}`
    ).join('\n\n---\n\n');

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
- INLINE CITATIONS REQUIRED: For every factual claim you make, append the corresponding source number in brackets, e.g., [1], [2], or [1,3]. Use the exact numbering provided in the context blocks above.

Answer:`;

    const references = topDocs.map((d, i) => ({
        id: i + 1,
        title: d.title,
        url: d.url
    }));
    
    yield JSON.stringify({ type: 'references', data: references }) + '\n\n';

    try {
        const streamRes = await callPythonStream(prompt);
        // We read the raw byte stream from "res" and yield it as chunks
        for await (const chunk of streamRes) {
            // The python server sends `data: {"chunk": "..."}\n\n`
            // But we actually only wanted to yield JSON string chunks in JS
            // Wait, Python already sends `data: ...` so we can just yield the raw chunk?
            // Actually streamController.js wraps my chunkStr: `res.write(`data: ${chunkStr}`);`
            // Meaning chunkStr is expected to be a JSON string like:
            // JSON.stringify({ type: 'chunk', data: text }) + '\n\n'
            
            // For simplicity, let's parse Python's outgoing SSE and re-emit:
            const str = chunk.toString();
            const lines = str.split('\n');
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    const dataObj = JSON.parse(line.substring(6));
                    if (dataObj.chunk) {
                        yield JSON.stringify({ type: 'chunk', data: dataObj.chunk }) + '\n\n';
                    } else if (dataObj.error) {
                        yield JSON.stringify({ type: 'error', data: dataObj.error }) + '\n\n';
                    }
                }
            }
        }
    } catch(err) {
        yield JSON.stringify({ type: 'error', data: err.message }) + '\n\n';
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
            let fixed = jsonStr.replace(/(\d),(\d{3}(?!\d))/g, '$1$2');

            // Fix trailing commas in objects or arrays
            fixed = fixed.replace(/,\s*([\]}])/g, '$1');

            return JSON.parse(fixed);
        } catch (err2) {
            if (jsonStr.startsWith('{') || jsonStr.startsWith('[')) {
                return { error: 'Invalid JSON format from LLM', raw: jsonStr.slice(0, 500) };
            }
            return jsonStr;
        }
    }
}

// ─── Knowledge Panel ──────────────────────────────────────────────────────────

async function extractKnowledgePanel(query, documents) {
    if (!documents || documents.length === 0) return null;
    const context = documents.slice(0, 4).map(d => `Source: ${d.url}\n${d.content.slice(0, 1000)}`).join('\n---\n');
    
    const prompt = `You are an entity extractor. Based on the query and context, extract key statistics, facts, and entities about the subject of the search into a valid JSON object.
Query: "${query}"
Context:
${context}

Instructions:
- Output ONLY a flat JSON object with 3 to 6 key attributes.
- Example keys: "Price", "Founders", "Release Date", "GitHub Stars", "License".
- If the context doesn't contain enough factual stats, return {}.
- Do not output markdown, just raw JSON.`;

    const result = await extract(prompt);
    if (!result || Object.keys(result).length === 0 || result.error) return null;
    return result;
}

// ─── Conversational Chat ──────────────────────────────────────────────────────

async function* chatWithContextStream(messages, documents) {
    const context = (documents || []).slice(0, 5).map((d, i) =>
        `[${i + 1}] Title: ${d.title}\nSource: ${d.url}\nContent: ${d.content.slice(0, 1500)}`
    ).join('\n\n---\n\n');

    let historyStr = messages.map(m => `${m.role.toUpperCase()}: ${m.content}`).join('\n');

    const prompt = `You are Searqon-AI, an interactive search assistant. 
Use the search context to answer the user's latest query in the conversation history.

Search Context:
${context}

Conversation History:
${historyStr}

Instructions:
- Provide a helpful, factual response to the latest USER message.
- If making a factual claim, cite the source number from the context using [1], [2], etc.
- Keep the answer concise.

ASSISTANT:`;

    try {
        const streamRes = await callPythonStream(prompt);
        for await (const chunk of streamRes) {
            const str = chunk.toString();
            const lines = str.split('\n');
            for (const line of lines) {
                if (line.startsWith('data: ')) {
                    const dataObj = JSON.parse(line.substring(6));
                    if (dataObj.chunk) {
                        yield JSON.stringify({ type: 'chunk', data: dataObj.chunk }) + '\n\n';
                    } else if (dataObj.error) {
                        yield JSON.stringify({ type: 'error', data: dataObj.error }) + '\n\n';
                    }
                }
            }
        }
    } catch(err) {
        yield JSON.stringify({ type: 'error', data: err.message }) + '\n\n';
    }
}

async function chatWithContext(messages, documents) {
    const context = (documents || []).slice(0, 5).map((d, i) =>
        `[${i + 1}] Title: ${d.title}\nSource: ${d.url}\nContent: ${d.content.slice(0, 1500)}`
    ).join('\n\n---\n\n');

    let historyStr = messages.map(m => `${m.role.toUpperCase()}: ${m.content}`).join('\n');

    const prompt = `You are Searqon-AI, an interactive search assistant. 
Use the search context to answer the user's latest query in the conversation history.

Search Context:
${context}

Conversation History:
${historyStr}

Instructions:
- Provide a helpful, factual response to the latest USER message.
- If making a factual claim, cite the source number from the context using [1], [2], etc.
- Keep the answer concise.

ASSISTANT:`;

    let answerText = '';
    switch (BACKEND) {
        case 'gemini':  answerText = await callGeminiRaw(prompt); break;
        case 'openai':  answerText = await callOpenAIRaw(prompt); break;
        case 'ollama':
        default:        answerText = await callOllamaRaw(prompt); break;
    }
    return answerText;
}

module.exports = { 
    extract, 
    synthesizeAnswer, 
    synthesizeAnswerStream, 
    extractKnowledgePanel, 
    chatWithContext,
    chatWithContextStream, 
    BACKEND 
};
