from rank_bm25 import BM25Okapi
from typing import Dict, List, Tuple

class BM25Retriever:
    def __init__(self, k1: float = 1.5, b: float = 0.75):
        self.k1 = k1
        self.b = b
        self.bm25 = None
        self.doc_ids = None
        self.tokenized_docs = None
    
    def fit(self, tokenized_docs: List[List[str]], doc_ids: List[str]):
        self.doc_ids = doc_ids
        self.tokenized_docs = tokenized_docs
        self.bm25 = BM25Okapi(tokenized_docs, k1=self.k1, b=self.b)
        print(f"BM25 fitted on {len(tokenized_docs)} documents")
    
    def retrieve(self, query_tokens: List[str], k: int = 10) -> List[Tuple[str, float]]:
        if self.bm25 is None:
            raise ValueError("Retriever not fitted")
        
        scores = self.bm25.get_scores(query_tokens)
        top_indices = sorted(range(len(scores)), key=lambda i: scores[i], reverse=True)[:k]
        results = [(self.doc_ids[i], float(scores[i])) for i in top_indices if scores[i] > 0]
        return results
