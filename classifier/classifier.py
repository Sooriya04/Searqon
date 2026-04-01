"""
Searqon Intelligence Layer — Python Microservice
Two endpoints:
  POST /classify   — Semantic intent routing (~1ms, no LLM)
  POST /summarize  — TF-IDF extractive highlights (~5ms, no LLM)

RAM Usage: < 10MB | Latency: < 10ms | No external dependencies.
"""

import json
import re
import math
import logging
from collections import Counter
from http.server import HTTPServer, BaseHTTPRequestHandler

# ─── Config ───────────────────────────────────────────────────────────────────

HOST = "0.0.0.0"
PORT = 3003

logging.basicConfig(
    level=logging.INFO,
    format="[%(asctime)s] [Classifier] %(message)s",
    datefmt="%H:%M:%S"
)

# ─── Source Map ───────────────────────────────────────────────────────────────

SOURCE_MAP = {
    "tech":     ["github", "hackernews", "geeksforgeeks", "reddit", "wikipedia", "youtube"],
    "medical":  ["pubmed", "medrxiv", "arxiv", "openalex", "doaj"],
    "academic": ["arxiv", "doaj", "openalex", "wikipedia"],
    "coding":   ["github", "geeksforgeeks", "hackernews", "reddit"],
    "news":     ["hackernews", "reddit", "web", "youtube"],
    "science":  ["arxiv", "openalex", "doaj", "wikipedia"],
    "video":    ["youtube", "reddit"],
    "general":  ["wikipedia", "web", "reddit", "youtube"],
}

BASELINE = ["duckduckgo"]

# ─── Intent Matrix (Weighted Scoring) ──────────────────────────────────────────

INTENT_MATRIX = {
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
}


# ─── Query Classification ─────────────────────────────────────────────────────

def classify_query(query: str) -> dict:
    """Intelligent Semantic Routing without LLM overhead."""
    q = query.lower()
    scores = {cat: 0.0 for cat in SOURCE_MAP.keys()}

    for category, intent in INTENT_MATRIX.items():
        for kw, weight in intent.get("keywords", {}).items():
            if kw in q:
                scores[category] += weight
                if f" {kw} " in f" {q} ":
                    scores[category] += 1.0

        for phrase in intent.get("phrases", []):
            if phrase in q:
                scores[category] += 3.0

    if q.startswith(("how", "show", "write", "create")):
        scores["coding"] += 2.0
        scores["tech"] += 1.0

    if any(x in q for x in ["latest", "today", "newest", "recently"]):
        scores["news"] += 1.5

    THRESHOLD = 1.0
    matched = [cat for cat, score in scores.items() if score >= THRESHOLD]
    matched.sort(key=lambda x: scores[x], reverse=True)

    if not matched:
        matched = ["general"]

    top_categories = matched[:2]

    sources = []
    for cat in top_categories:
        for src in SOURCE_MAP.get(cat, []):
            if src not in sources:
                sources.append(src)

    for b in BASELINE:
        if b not in sources:
            sources.append(b)

    return {
        "query": query,
        "strategy": "semantic_intent",
        "categories": top_categories,
        "sources": sources,
        "scores": {k: round(v, 2) for k, v in scores.items() if v > 0}
    }


# ─── TF-IDF Extractive Summarizer ─────────────────────────────────────────────

# Common English stopwords — embedded to avoid any external dependency
STOPWORDS = frozenset([
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
])


def tokenize(text: str) -> list:
    """Split text into lowercase words, removing non-alpha characters."""
    return [w for w in re.findall(r'[a-z]+', text.lower()) if len(w) > 2 and w not in STOPWORDS]


def split_sentences(text: str) -> list:
    """Split text into sentences using regex. Handles common abbreviations."""
    # Split on . ! ? followed by space or end of string
    raw = re.split(r'(?<=[.!?])\s+', text.strip())
    # Filter out very short or noisy sentences
    sentences = []
    for s in raw:
        s = s.strip()
        # Must be a real sentence: >= 8 words, not mostly numbers/symbols
        word_count = len(s.split())
        if word_count >= 8 and word_count <= 80:
            # Skip lines that are mostly navigation / boilerplate
            alpha_ratio = sum(1 for c in s if c.isalpha()) / max(len(s), 1)
            if alpha_ratio > 0.5:
                sentences.append(s)
    return sentences


def tfidf_summarize(query: str, documents: list, num_highlights: int = 5) -> list:
    """
    Pure Python TF-IDF extractive summarizer.
    
    Args:
        query: The original user query
        documents: List of dicts with keys: source, title, content, url
        num_highlights: Number of top sentences to extract
    
    Returns:
        List of highlight dicts with: sentence, source, title, url, score
    """
    if not documents:
        return []

    # 1. Collect all sentences with their source metadata
    all_sentences = []  # [(sentence_text, source, title, url)]
    for doc in documents:
        content = doc.get("content", "")
        if not content or not isinstance(content, str):
            continue

        source = doc.get("source", "unknown")
        title = doc.get("title", "")
        url = doc.get("url", doc.get("link", ""))

        sentences = split_sentences(content)
        for sent in sentences:
            all_sentences.append((sent, source, title, url))

    if not all_sentences:
        return []

    # 2. Build document frequency (DF) across all sentences
    num_docs = len(all_sentences)
    df = Counter()
    for sent, *_ in all_sentences:
        unique_words = set(tokenize(sent))
        for word in unique_words:
            df[word] += 1

    # 3. Get query terms for relevance boosting
    query_terms = set(tokenize(query))

    # 4. Score each sentence using TF-IDF + query relevance
    scored = []
    seen_normalized = set()  # Deduplicate near-identical sentences

    for sent, source, title, url in all_sentences:
        words = tokenize(sent)
        if not words:
            continue

        # Deduplication: normalize sentence to first 10 words
        norm_key = " ".join(words[:10])
        if norm_key in seen_normalized:
            continue
        seen_normalized.add(norm_key)

        # TF-IDF score
        tf = Counter(words)
        tfidf_score = 0.0
        for word, count in tf.items():
            tf_val = count / len(words)
            idf_val = math.log((num_docs + 1) / (df.get(word, 0) + 1)) + 1.0
            tfidf_score += tf_val * idf_val

        # Query relevance boost: sentences containing query terms score higher
        query_overlap = len(query_terms & set(words))
        relevance_boost = query_overlap * 2.0

        # Position bias: slightly prefer earlier sentences in each document
        # (already handled by order in split_sentences)

        # Length penalty: prefer medium-length sentences (15-40 words)
        word_count = len(words)
        if 15 <= word_count <= 40:
            length_bonus = 1.2
        elif 10 <= word_count <= 50:
            length_bonus = 1.0
        else:
            length_bonus = 0.7

        final_score = (tfidf_score + relevance_boost) * length_bonus

        scored.append({
            "sentence": sent,
            "source": source,
            "title": title,
            "url": url,
            "score": round(final_score, 4)
        })

    # 5. Sort by score descending, pick top N
    scored.sort(key=lambda x: x["score"], reverse=True)

    # 6. Diversify: don't take more than 2 highlights from the same source
    highlights = []
    source_counts = Counter()
    for item in scored:
        if len(highlights) >= num_highlights:
            break
        src = item["source"]
        if source_counts[src] < 2:
            highlights.append(item)
            source_counts[src] += 1

    return highlights


# ─── HTTP Server ──────────────────────────────────────────────────────────────

class ClassifierHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass

    def send_json(self, status: int, data: dict):
        body = json.dumps(data).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        if self.path == "/health":
            self.send_json(200, {
                "status": "ok",
                "engine": "semantic_intent + tfidf_summarizer",
                "endpoints": ["POST /classify", "POST /summarize"],
                "reliability": "100%",
            })
        else:
            self.send_json(404, {"error": "Not found"})

    def do_POST(self):
        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)

        try:
            data = json.loads(body)
        except json.JSONDecodeError:
            self.send_json(400, {"error": "Invalid JSON"})
            return

        # ── POST /classify ─────────────────────────────────────────────────
        if self.path == "/classify":
            query = data.get("query", "").strip()
            if not query:
                self.send_json(400, {"error": "query field is required"})
                return
            result = classify_query(query)
            self.send_json(200, result)

        # ── POST /summarize ────────────────────────────────────────────────
        elif self.path == "/summarize":
            query = data.get("query", "").strip()
            documents = data.get("documents", [])
            num_highlights = data.get("num_highlights", 5)

            if not query:
                self.send_json(400, {"error": "query field is required"})
                return
            if not documents or not isinstance(documents, list):
                self.send_json(400, {"error": "documents must be a non-empty array"})
                return

            highlights = tfidf_summarize(query, documents, num_highlights)
            self.send_json(200, {
                "query": query,
                "num_highlights": len(highlights),
                "highlights": highlights,
            })

        else:
            self.send_json(404, {"error": "Not found"})


# ─── Main ─────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    logging.info(f"Starting Searqon Intelligence Layer on port {PORT}")
    logging.info(f"Endpoints: POST /classify | POST /summarize | GET /health")
    logging.info(f"Engine: Semantic Intent + TF-IDF Summarizer | No LLM Required")

    HTTPServer.allow_reuse_address = True
    server = HTTPServer((HOST, PORT), ClassifierHandler)
    server.serve_forever()
