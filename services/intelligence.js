/**
 * Searqon Intelligence Layer — JavaScript Migration
 * Logic ported from Python to reduce memory overhead.
 */

const SOURCE_MAP = {
    "tech":     ["github", "hackernews", "geeksforgeeks", "reddit", "wikipedia", "youtube"],
    "medical":  ["pubmed", "medrxiv", "arxiv", "openalex", "doaj"],
    "academic": ["arxiv", "doaj", "openalex", "wikipedia"],
    "coding":   ["github", "geeksforgeeks", "hackernews", "reddit"],
    "news":     ["hackernews", "reddit", "web", "youtube"],
    "science":  ["arxiv", "openalex", "doaj", "wikipedia"],
    "video":    ["youtube", "reddit"],
    "general":  ["wikipedia", "web", "reddit", "youtube"],
};

const BASELINE = ["duckduckgo"];

const INTENT_MATRIX = {
    "tech": {
        "keywords": {
            "api": 2.5, "software": 1.5, "framework": 2.0, "library": 1.5,
            "git": 2.0, "github": 3.0, "npm": 2.5, "package": 2.0,
            "python": 2.0, "javascript": 2.0, "typescript": 2.0, "rust": 2.5, "golang": 3.0,
            "react": 2.0, "node": 2.0, "backend": 2.0, "frontend": 2.0,
            "linux": 1.5, "terminal": 2.0, "bash": 2.0, "docker": 2.5, "kubernetes": 3.0,
            "cloud": 1.5, "aws": 2.5, "azure": 2.5, "gcp": 2.5, "deploy": 2.0,
            "claude": 3.0, "gpt": 2.5, "llm": 2.5, "anthropic": 3.0, "openai": 2.5,
            "vector db": 3.0, "inference": 2.5, "token": 1.5, "gpu": 1.5,
        },
        "phrases": ["open source", "cli tool", "error log", "build failed", "code example"]
    },
    "medical": {
        "keywords": {
            "disease": 2.5, "disorder": 2.5, "medicine": 2.0, "drug": 2.5, "medication": 2.5,
            "symptom": 2.5, "clinical": 2.0, "health": 1.0, "patient": 2.0, "doctor": 1.5,
            "vaccine": 3.0, "surgery": 2.5, "therapy": 2.0, "diagnosis": 3.0, "anatomy": 2.0,
            "pathology": 3.0, "biomedical": 2.0, "side effect": 3.0, "paracetamol": 3.0,
            "aspirin": 3.0, "ibuprofen": 3.0, "cancer": 2.5, "tumor": 2.5, "diabetes": 2.5,
            "heart": 1.5, "blood": 1.5, "infection": 2.0, "antibiotic": 2.5, "psychiatric": 2.5,
        }
    },
    "academic": {
        "keywords": {
            "research": 2.0, "paper": 2.5, "study": 1.5, "analysis": 1.5, "theory": 1.5,
            "peer review": 3.0, "scientific": 2.0, "physics": 1.5, "chemistry": 1.5,
            "biology": 1.5, "calculus": 2.5, "arxiv": 3.0, "scholar": 2.0, "abstract": 2.5,
            "citation": 3.0, "literature": 2.0, "hypothesis": 2.5, "data science": 2.0,
            "statistics": 2.0, "deep learning": 2.0,
        }
    },
    "coding": {
        "keywords": {
            "how to": 3.0, "tutorial": 2.5, "example": 2.0, "implement": 2.5, "function": 2.5,
            "class": 2.0, "method": 2.0, "algorithm": 3.0, "sorting": 2.5, "recursion": 3.0,
            "loop": 2.0, "array": 1.5, "pointer": 3.0, "heap": 2.5, "stack": 1.5, "graph": 2.0,
        }
    },
    "news": {
        "keywords": {
            "news": 3.0, "latest": 2.0, "today": 1.5, "update": 1.5, "announcement": 2.5,
            "trending": 2.0, "breaking": 3.0, "headline": 2.5, "recent": 1.5, "startup": 2.0,
        }
    }
};

const STOPWORDS = new Set([
    "a", "about", "above", "after", "again", "against", "all", "am", "an", "and",
    "any", "are", "aren't", "as", "at", "be", "because", "been", "before", "being",
    "below", "between", "both", "but", "by", "can't", "cannot", "could", "couldn't",
    "did", "didn't", "do", "does", "doesn't", "doing", "don't", "down", "during",
    "each", "few", "for", "from", "further", "get", "got", "had", "hadn't", "has",
    "hasn't", "have", "haven't", "having", "he", "her", "here", "hers", "herself",
    "him", "himself", "his", "how", "i", "if", "in", "into", "is", "isn't", "it",
    "its", "itself", "just", "let's", "me", "might", "more", "most", "mustn't", "my",
    "myself", "no", "nor", "not", "of", "off", "on", "once", "only", "or", "other",
    "ought", "our", "ours", "ourselves", "out", "over", "own", "same", "shan't",
    "she", "should", "shouldn't", "so", "some", "such", "than", "that", "the",
    "their", "theirs", "them", "themselves", "then", "there", "these", "they",
    "this", "those", "through", "to", "too", "under", "until", "up", "very", "was",
    "wasn't", "we", "were", "weren't", "what", "when", "where", "which", "while",
    "who", "whom", "why", "will", "with", "won't", "would", "wouldn't", "you",
    "your", "yours", "yourself", "yourselves", "also", "like", "one", "use", "used",
    "using", "many", "may", "much", "well", "new", "first", "two", "way", "make",
    "see", "know", "take", "come", "made", "find", "back", "still", "even", "give",
]);

/**
 * Ported from classifier.py: classifyQuery
 */
function classifyQuery(query) {
    const q = query.toLowerCase();
    const scores = {};
    Object.keys(SOURCE_MAP).forEach(cat => scores[cat] = 0.0);

    for (const [category, intent] of Object.entries(INTENT_MATRIX)) {
        if (intent.keywords) {
            for (const [kw, weight] of Object.entries(intent.keywords)) {
                if (q.includes(kw)) {
                    scores[category] += weight;
                    if (new RegExp(`\\b${kw}\\b`).test(q)) {
                        scores[category] += 1.0;
                    }
                }
            }
        }
        if (intent.phrases) {
            for (const phrase of intent.phrases) {
                if (q.includes(phrase)) {
                    scores[category] += 3.0;
                }
            }
        }
    }

    if (q.startsWith("how") || q.startsWith("show") || q.startsWith("write") || q.startsWith("create")) {
        scores["coding"] += 2.0;
        scores["tech"] += 1.0;
    }

    if (["latest", "today", "newest", "recently"].some(x => q.includes(x))) {
        scores["news"] += 1.5;
    }

    const THRESHOLD = 1.0;
    let matched = Object.keys(scores).filter(cat => scores[cat] >= THRESHOLD);
    matched.sort((a, b) => scores[b] - scores[a]);

    if (matched.length === 0) {
        matched = ["general"];
    }

    const topCategories = matched.slice(0, 2);
    const sources = [];

    for (const cat of topCategories) {
        for (const src of SOURCE_MAP[cat] || []) {
            if (!sources.includes(src)) sources.push(src);
        }
    }

    for (const b of BASELINE) {
        if (!sources.includes(b)) sources.push(b);
    }

    return {
        query,
        strategy: "semantic_intent",
        categories: topCategories,
        sources: sources,
        scores: Object.fromEntries(Object.entries(scores).filter(([_, v]) => v > 0).map(([k, v]) => [k, Math.round(v * 100) / 100]))
    };
}

/**
 * Ported from classifier.py: tfidfSummarize
 */
function tfidfSummarize(query, documents, numHighlights = 5) {
    if (!documents || documents.length === 0) return [];

    const tokenize = (text) => {
        return (text.toLowerCase().match(/[a-z]+/g) || [])
            .filter(w => w.length > 2 && !STOPWORDS.has(w));
    };

    const splitSentences = (text) => {
        if (!text) return [];
        const raw = text.trim().split(/(?<=[.!?])\s+/);
        const sentences = [];
        for (let s of raw) {
            s = s.trim();
            const words = s.split(/\s+/);
            if (words.length >= 8 && words.length <= 80) {
                const alphaCount = (s.match(/[a-zA-Z]/g) || []).length;
                if (alphaCount / s.length > 0.5) {
                    sentences.push(s);
                }
            }
        }
        return sentences;
    };

    const allSentences = [];
    for (const doc of documents) {
        const sentences = splitSentences(doc.content);
        for (const sent of sentences) {
            allSentences.push({
                sentence: sent,
                source: doc.source || "unknown",
                title: doc.title || "",
                url: doc.url || doc.link || ""
            });
        }
    }

    if (allSentences.length === 0) return [];

    const numDocs = allSentences.length;
    const df = {};
    for (const item of allSentences) {
        const uniqueWords = new Set(tokenize(item.sentence));
        uniqueWords.forEach(word => {
            df[word] = (df[word] || 0) + 1;
        });
    }

    const queryTerms = new Set(tokenize(query));
    const scored = [];
    const seenNormalized = new Set();

    for (const item of allSentences) {
        const words = tokenize(item.sentence);
        if (words.length === 0) continue;

        const normKey = words.slice(0, 10).join(" ");
        if (seenNormalized.has(normKey)) continue;
        seenNormalized.add(normKey);

        const tf = {};
        words.forEach(w => tf[w] = (tf[w] || 0) + 1);

        let tfidfScore = 0.0;
        for (const [word, count] of Object.entries(tf)) {
            const tfVal = count / words.length;
            const idfVal = Math.log((numDocs + 1) / ((df[word] || 0) + 1)) + 1.0;
            tfidfScore += tfVal * idfVal;
        }

        const queryOverlap = words.filter(w => queryTerms.has(w)).length;
        const relevanceBoost = queryOverlap * 2.0;

        let lengthBonus = 0.7;
        const wordCount = words.length;
        if (wordCount >= 15 && wordCount <= 40) lengthBonus = 1.2;
        else if (wordCount >= 10 && wordCount <= 50) lengthBonus = 1.0;

        const finalScore = (tfidfScore + relevanceBoost) * lengthBonus;

        scored.push({
            ...item,
            score: Math.round(finalScore * 10000) / 10000
        });
    }

    scored.sort((a, b) => b.score - a.score);

    const highlights = [];
    const sourceCounts = {};
    for (const item of scored) {
        if (highlights.length >= numHighlights) break;
        const src = item.source;
        sourceCounts[src] = (sourceCounts[src] || 0);
        if (sourceCounts[src] < 2) {
            highlights.push(item);
            sourceCounts[src]++;
        }
    }

    return highlights;
}

module.exports = { classifyQuery, tfidfSummarize };
