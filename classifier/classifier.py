"""
Searqon Query Classifier — Python Microservice
Uses Ollama (qwen2.5:0.5b) to classify user queries into search categories.
Falls back to keyword matching if Ollama is unavailable.
"""

import json
import re
import logging
from http.server import HTTPServer, BaseHTTPRequestHandler

# Try to import ollama — it's optional (fallback will handle the rest)
try:
    import ollama
    OLLAMA_AVAILABLE = True
except ImportError:
    OLLAMA_AVAILABLE = False
    logging.warning("[Classifier] ollama package not installed — using keyword fallback only")

# ─── Config ───────────────────────────────────────────────────────────────────

HOST = "0.0.0.0"
PORT = 3003
OLLAMA_MODEL = "qwen2.5:0.5b"

logging.basicConfig(
    level=logging.INFO,
    format="[%(asctime)s] [Classifier] %(message)s",
    datefmt="%H:%M:%S"
)

# ─── Source Map ───────────────────────────────────────────────────────────────

# All available sources in Searqon
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

# Always appended at the end — baseline fallback as per design
BASELINE = ["duckduckgo"]

# ─── Keyword Fallback ─────────────────────────────────────────────────────────

KEYWORD_MAP = {
    "tech": [
        "code", "coding", "programming", "software", "api", "framework", "library",
        "git", "github", "npm", "package", "python", "javascript", "typescript",
        "react", "vue", "angular", "node", "backend", "frontend", "database", "sql",
        "linux", "terminal", "bash", "docker", "kubernetes", "cloud", "aws", "azure",
        "gcp", "deploy", "devops", "cli", "error", "exception", "debug", "implement",
        "claude", "gpt", "llm", "ai model", "inference", "token", "claude code",
        "openai", "anthropic", "gemini", "ollama", "langchain", "vector db",
    ],
    "medical": [
        "disease", "disorder", "medicine", "drug", "medication", "treatment", "symptom",
        "clinical", "health", "patient", "doctor", "physician", "pharmacy", "vaccine",
        "surgery", "therapy", "diagnosis", "anatomy", "physiology", "pathology",
        "biomedical", "genetic", "pharmacology", "side effect", "paracetamol",
        "aspirin", "ibuprofen", "cancer", "tumor", "diabetes", "heart", "blood",
        "infection", "antibiotic", "psychiatric", "mental health", "neurology",
    ],
    "academic": [
        "research", "paper", "study", "analysis", "theory", "methodology",
        "experiment", "journal", "peer review", "scientific", "physics", "chemistry",
        "biology", "mathematics", "math", "algebra", "calculus", "quantum",
        "arxiv", "scholar", "abstract", "citation", "literature", "review",
        "hypothesis", "data science", "statistics", "machine learning", "deep learning",
    ],
    "coding": [
        "how to", "tutorial", "example", "implement", "function", "class", "method",
        "algorithm", "data structure", "sorting", "recursion", "loop", "array",
        "object", "pointer", "heap", "stack", "queue", "graph", "tree",
    ],
    "news": [
        "news", "latest", "today", "update", "announcement", "trending", "current",
        "breaking", "headline", "recent", "event", "hacker news", "startup",
    ],
    "video": [
        "video", "youtube", "tutorial video", "watch", "channel", "playlist",
        "explanation video", "demo",
    ],
    "science": [
        "physics", "chemistry", "biology", "space", "astronomy", "cosmology",
        "climate", "environment", "evolution", "neuroscience", "genetics",
        "particle", "quantum", "relativity", "gravitational",
    ],
}


def keyword_classify(query: str):
    """Classify query using keyword matching — instant, no LLM."""
    q = query.lower()
    matched = set()

    for category, keywords in KEYWORD_MAP.items():
        if any(kw in q for kw in keywords):
            matched.add(category)

    if not matched:
        matched.add("general")

    sources = []
    for cat in matched:
        for src in SOURCE_MAP.get(cat, []):
            if src not in sources:
                sources.append(src)

    # DuckDuckGo always last
    for b in BASELINE:
        if b not in sources:
            sources.append(b)

    return sources


# ─── LLM Classification ───────────────────────────────────────────────────────

SYSTEM_PROMPT = """Classify the user query into search categories. Reply ONLY with a JSON array.

Valid categories (use EXACTLY these strings):
- "tech" — programming, software, AI/LLM, APIs, developer tools, cloud, CLI tools
- "medical" — drugs, diseases, symptoms, clinical research, health conditions
- "academic" — scientific research, papers, physics, chemistry, biology, math
- "coding" — code examples, algorithms, tutorials, how-to implementations
- "news" — current events, announcements, trending, startups
- "video" — video tutorials, YouTube, demos
- "science" — physics, space, climate, evolution, quantum, genetics
- "general" — everything else

Rules:
1. Reply ONLY with a valid JSON array using the exact category strings above.
2. Pick 1 to 3 categories max.
3. No explanation. No markdown. Just the JSON array.

Examples:
"how to use react hooks" → ["tech", "coding"]
"side effects of paracetamol" → ["medical"]
"latest OpenAI news" → ["tech", "news"]
"quantum entanglement" → ["science", "academic"]
"best pasta recipe" → ["general"]
"why rust is the best programming language" → ["tech", "coding"]
"""

# Alias map — normalize non-standard category names from the LLM
CATEGORY_ALIASES = {
    "programming":  "tech",
    "software":     "tech",
    "developer":    "tech",
    "development":  "tech",
    "engineering":  "tech",
    "health":       "medical",
    "medicine":     "medical",
    "biology":      "science",
    "physics":      "science",
    "chemistry":    "science",
    "research":     "academic",
    "science":      "science",
    "tutorial":     "coding",
}


def llm_classify(query: str):
    """Classify using Ollama qwen2.5:0.5b. Returns None if unavailable."""
    if not OLLAMA_AVAILABLE:
        return None

    try:
        response = ollama.chat(
            model=OLLAMA_MODEL,
            messages=[
                {"role": "system", "content": SYSTEM_PROMPT},
                {"role": "user", "content": f'Query: "{query}"'},
            ],
            options={"temperature": 0.0, "num_predict": 64},
        )

        # ollama SDK v0.6+ returns a Pydantic object — use attribute access
        text = response.message.content.strip()
        logging.info(f"[Classifier] LLM raw output: {text!r}")

        # Parse the JSON array from the response
        match = re.search(r'\[.*?\]', text, re.DOTALL)
        if not match:
            logging.warning(f"[Classifier] Bad LLM output — falling back to keywords")
            return None

        categories = json.loads(match.group())

        # Validate — normalize aliases and only allow known categories
        valid = set(SOURCE_MAP.keys())
        normalized = []
        for c in categories:
            c = c.lower()
            c = CATEGORY_ALIASES.get(c, c)  # normalize aliases
            if c in valid and c not in normalized:
                normalized.append(c)
        categories = normalized

        if not categories:
            logging.warning(f"[Classifier] LLM returned no valid categories")
            return None

        logging.info(f"[Classifier] LLM for '{query}' → {categories}")
        return categories

    except Exception as e:
        logging.warning(f"[Classifier] LLM error: {e}")
        return None


def classify_and_route(query: str) -> dict:
    """Main classification pipeline."""
    strategy = "llm"

    # Try LLM first
    categories = llm_classify(query)

    if categories is None:
        # Fallback to keyword-based
        strategy = "keyword_fallback"
        sources = keyword_classify(query)
        logging.info(f"[Classifier] Keyword fallback for '{query}' → {sources}")
        return {
            "query": query,
            "strategy": strategy,
            "categories": [],
            "sources": sources,
            "baseline": BASELINE,
        }

    # Map categories to sources
    sources = []
    for cat in categories:
        for src in SOURCE_MAP.get(cat, []):
            if src not in sources:
                sources.append(src)

    # Always add DuckDuckGo at the end
    for b in BASELINE:
        if b not in sources:
            sources.append(b)

    return {
        "query": query,
        "strategy": strategy,
        "categories": categories,
        "sources": sources,
        "baseline": BASELINE,
    }


# ─── HTTP Server ──────────────────────────────────────────────────────────────

class ClassifierHandler(BaseHTTPRequestHandler):
    def log_message(self, format, *args):
        pass  # Suppress default HTTP logs (we use our own)

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
                "model": OLLAMA_MODEL,
                "ollama_available": OLLAMA_AVAILABLE,
                "categories": list(SOURCE_MAP.keys()),
            })
        else:
            self.send_json(404, {"error": "Not found"})

    def do_POST(self):
        if self.path != "/classify":
            self.send_json(404, {"error": "Not found"})
            return

        length = int(self.headers.get("Content-Length", 0))
        body = self.rfile.read(length)

        try:
            data = json.loads(body)
        except json.JSONDecodeError:
            self.send_json(400, {"error": "Invalid JSON"})
            return

        query = data.get("query", "").strip()
        if not query:
            self.send_json(400, {"error": "query field is required"})
            return

        result = classify_and_route(query)
        self.send_json(200, result)


# ─── Main ─────────────────────────────────────────────────────────────────────

if __name__ == "__main__":
    logging.info(f"Starting on port {PORT}")
    logging.info(f"Ollama model: {OLLAMA_MODEL} | Available: {OLLAMA_AVAILABLE}")
    logging.info(f"Endpoints: POST /classify | GET /health")

    server = HTTPServer((HOST, PORT), ClassifierHandler)
    server.serve_forever()
