import numpy as np
from sentence_transformers import SentenceTransformer
from sklearn.metrics.pairwise import cosine_similarity
from typing import Dict, List, Tuple

class SemanticReranker:
    def __init__(self, model_name: str = "all-MiniLM-L6-v2"):
        print(f"Loading {model_name}...")
        self.model = SentenceTransformer(model_name)
    
    def embed_texts(self, texts: List[str], batch_size: int = 32) -> np.ndarray:
        embeddings = self.model.encode(texts, batch_size=batch_size, show_progress_bar=False)
        return np.array(embeddings)
    
    def rerank(self, query: str, candidates: List[Tuple[str, float]], 
               doc_texts: Dict[str, str], k: int = 10) -> List[Tuple[str, float]]:
        if not candidates:
            return []
        
        doc_ids = [doc_id for doc_id, _ in candidates]
        texts = [doc_texts[doc_id] for doc_id in doc_ids]
        
        query_embedding = self.model.encode([query], show_progress_bar=False)[0]
        doc_embeddings = self.model.encode(texts, batch_size=32, show_progress_bar=False)
        
        similarities = cosine_similarity([query_embedding], doc_embeddings)[0]
        
        reranked = [(doc_ids[i], float(similarities[i])) for i in range(len(doc_ids))]
        reranked.sort(key=lambda x: x[1], reverse=True)
        
        return reranked[:k]
