import numpy as np
from typing import Dict, List, Tuple
from src.utils import normalize_scores, explain_score

class HybridRanker:
    def __init__(self, alpha: float = 0.5, beta: float = 0.3, gamma: float = 0.2):
        if not np.isclose(alpha + beta + gamma, 1.0):
            raise ValueError(f"Weights must sum to 1.0")
        self.alpha = alpha
        self.beta = beta
        self.gamma = gamma
    
    def rank(self, bm25_results: List[Tuple[str, float]],
             semantic_results: List[Tuple[str, float]],
             credibility_scores: Dict[str, float]) -> List[Tuple[str, float, Dict]]:
        bm25_dict = dict(bm25_results)
        semantic_dict = dict(semantic_results)
        
        all_docs = list(set(bm25_dict.keys()) | set(semantic_dict.keys()))
        
        bm25_scores = np.array([bm25_dict.get(doc_id, 0.0) for doc_id in all_docs])
        semantic_scores = np.array([semantic_dict.get(doc_id, 0.0) for doc_id in all_docs])
        credibility_vec = np.array([credibility_scores.get(doc_id, 0.5) for doc_id in all_docs])
        
        bm25_scores_norm = normalize_scores(bm25_scores) if np.max(bm25_scores) > 0 else bm25_scores
        semantic_scores_norm = normalize_scores(semantic_scores) if np.max(semantic_scores) > 0 else semantic_scores
        credibility_norm = normalize_scores(credibility_vec) if np.max(credibility_vec) > 0 else credibility_vec
        
        final_scores = (self.alpha * bm25_scores_norm + self.beta * semantic_scores_norm + self.gamma * credibility_norm)
        
        results = []
        for i, doc_id in enumerate(all_docs):
            if final_scores[i] > 0:
                component_scores = {
                    'bm25': float(bm25_dict.get(doc_id, 0.0)),
                    'semantic': float(semantic_dict.get(doc_id, 0.0)),
                    'credibility': credibility_scores.get(doc_id, 0.5),
                    'bm25_norm': float(bm25_scores_norm[i]),
                    'semantic_norm': float(semantic_scores_norm[i]),
                    'credibility_norm': float(credibility_norm[i])
                }
                results.append((doc_id, float(final_scores[i]), component_scores))
        
        results.sort(key=lambda x: x[1], reverse=True)
        return results

class RankingExplainer:
    @staticmethod
    def explain_result(doc_id: str, rank: int, final_score: float, component_scores: Dict[str, float]) -> str:
        bm25_norm = component_scores.get('bm25_norm', 0.0)
        semantic_norm = component_scores.get('semantic_norm', 0.0)
        credibility_norm = component_scores.get('credibility_norm', 0.5)
        return explain_score(bm25_norm, semantic_norm, credibility_norm)
    
    @staticmethod
    def format_results(ranked_results: List[Tuple[str, float, Dict]], documents: Dict[str, Dict], limit: int = 5) -> List[Dict]:
        formatted = []
        for rank, (doc_id, final_score, component_scores) in enumerate(ranked_results[:limit], 1):
            doc = documents.get(doc_id, {})
            explanation = RankingExplainer.explain_result(doc_id, rank, final_score, component_scores)
            result_dict = {
                'rank': rank,
                'doc_id': doc_id,
                'title': doc.get('title', 'N/A'),
                'source': doc.get('source', 'N/A'),
                'final_score': round(final_score, 4),
                'bm25_score': round(component_scores.get('bm25', 0.0), 4),
                'semantic_score': round(component_scores.get('semantic', 0.0), 4),
                'credibility_score': round(component_scores.get('credibility', 0.5), 4),
                'explanation': explanation
            }
            formatted.append(result_dict)
        return formatted
