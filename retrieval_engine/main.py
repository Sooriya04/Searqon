from fastapi import FastAPI, HTTPException
from pydantic import BaseModel
from typing import List, Optional, Dict
import uvicorn
import time
from src.preprocess import TextPreprocessor
from src.bm25_retriever import BM25Retriever
from src.semantic_reranker import SemanticReranker
from src.credibility import CredibilityScorer
from src.ranker import HybridRanker, RankingExplainer

# Initialize models once at startup
print("Initializing Intelligent Reranker Engine...")
preprocessor = TextPreprocessor()
bm25_retriever = BM25Retriever()
semantic_reranker = SemanticReranker() # Uses all-MiniLM-L6-v2 by default
credibility_scorer = CredibilityScorer()
ranker = HybridRanker(alpha=0.3, beta=0.5, gamma=0.2)

app = FastAPI(title="Hybrid IR Reranker API")

class SearchDocument(BaseModel):
    title: str
    url: str
    content: str
    source: str

class RerankRequest(BaseModel):
    query: str
    documents: List[SearchDocument]
    limit: Optional[int] = 10

@app.post("/rerank")
async def rerank(request: RerankRequest):
    if not request.documents:
        return []
    
    start_time = time.time()
    
    # 1. Preprocessing
    query_tokens = preprocessor.preprocess(request.query)
    
    # Prepare documents for scoring
    # We use a dict with index as ID for convenience
    doc_dict = {}
    doc_texts = {} # For semantic reranker
    tokenized_docs = []
    doc_ids = []
    
    for i, doc in enumerate(request.documents):
        doc_id = f"doc_{i}"
        doc_dict[doc_id] = {
            'title': doc.title,
            'content': doc.content,
            'source': doc.source,
            'url': doc.url
        }
        full_text = f"{doc.title} {doc.content}"
        doc_texts[doc_id] = full_text
        doc_ids.append(doc_id)
        tokenized_docs.append(preprocessor.preprocess(full_text))

    # 2. BM25 Scoring
    bm25_retriever.fit(tokenized_docs, doc_ids)
    bm25_results = bm25_retriever.retrieve(query_tokens, k=len(doc_ids))
    
    # 3. Semantic Scoring
    # candidates list is expected as Tuple[str, float]
    initial_candidates = [(doc_id, 1.0) for doc_id in doc_ids] 
    semantic_results = semantic_reranker.rerank(request.query, initial_candidates, doc_texts, k=len(doc_ids))
    
    # 4. Credibility Scoring
    credibility_scores = credibility_scorer.score_documents(doc_dict)
    
    # 5. Hybrid Ranking
    ranked_results = ranker.rank(bm25_results, semantic_results, credibility_scores)
    
    # 6. Format and Explain
    final_output = []
    for rank_idx, (doc_id, score, components) in enumerate(ranked_results[:request.limit], 1):
        orig_idx = int(doc_id.split('_')[1])
        doc = request.documents[orig_idx]
        
        explanation = RankingExplainer.explain_result(doc_id, rank_idx, score, components)
        
        final_output.append({
            "title": doc.title,
            "url": doc.url,
            "content": doc.content,
            "source": doc.source,
            "score": round(score, 4),
            "explanation": explanation,
            "breakdown": {
                "bm25": round(components.get('bm25_norm', 0.0), 3),
                "semantic": round(components.get('semantic_norm', 0.0), 3),
                "credibility": round(components.get('credibility_norm', 0.0), 3)
            }
        })

    duration = time.time() - start_time
    print(f"Reranked {len(request.documents)} documents in {duration:.4f}s")
    
    return final_output

@app.get("/health")
async def health():
    return {"status": "healthy"}

if __name__ == "__main__":
    uvicorn.run(app, host="0.0.0.0", port=8001)
