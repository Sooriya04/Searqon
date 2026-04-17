import numpy as np
from typing import Dict, List, Tuple

def normalize_scores(scores: np.ndarray) -> np.ndarray:
    if len(scores) == 0:
        return scores
    
    min_score = np.min(scores)
    max_score = np.max(scores)
    
    if max_score == min_score:
        return np.ones_like(scores, dtype=float)
    
    return (scores - min_score) / (max_score - min_score)

def get_top_k(scores: Dict[str, float], k: int = 10) -> List[Tuple[str, float]]:
    sorted_scores = sorted(scores.items(), key=lambda x: x[1], reverse=True)
    return sorted_scores[:k]

def compute_ndcg(rankings: List[float], ideal_rankings: List[float], k: int = 10) -> float:
    dcg = 0.0
    idcg = 0.0
    
    for i in range(min(k, len(rankings))):
        if rankings[i] > 0:
            dcg += rankings[i] / np.log2(i + 2)
    
    for i in range(min(k, len(ideal_rankings))):
        if ideal_rankings[i] > 0:
            idcg += ideal_rankings[i] / np.log2(i + 2)
    
    if idcg == 0:
        return 0.0
    
    return dcg / idcg

def compute_precision_recall(retrieved: List[str], relevant: List[str], k: int = 10) -> Tuple[float, float]:
    retrieved_k = set(retrieved[:k])
    relevant_set = set(relevant)
    
    intersection = len(retrieved_k & relevant_set)
    
    precision = intersection / k if k > 0 else 0.0
    recall = intersection / len(relevant_set) if len(relevant_set) > 0 else 0.0
    
    return precision, recall

def compute_map(ranked_results: List[List[str]], qrels: Dict[str, List[str]]) -> float:
    if not ranked_results:
        return 0.0
    
    average_precisions = []
    
    for query_idx, ranked_docs in enumerate(ranked_results):
        query_id = f"q{query_idx+1:03d}"
        relevant = set(qrels.get(query_id, []))
        
        if not relevant:
            continue
        
        score = 0.0
        num_relevant = 0
        
        for rank, doc_id in enumerate(ranked_docs, 1):
            if doc_id in relevant:
                num_relevant += 1
                score += num_relevant / rank
        
        average_precisions.append(score / len(relevant))
    
    return np.mean(average_precisions) if average_precisions else 0.0

def explain_score(bm25_score: float, semantic_score: float, credibility_score: float,
                  alpha: float = 0.5, beta: float = 0.3, gamma: float = 0.2) -> str:
    explanations = []
    
    if bm25_score > 0.7:
        explanations.append("strong lexical match")
    elif bm25_score > 0.4:
        explanations.append("moderate lexical match")
    else:
        explanations.append("weak lexical match")
    
    if semantic_score > 0.7:
        explanations.append("high semantic similarity")
    elif semantic_score > 0.4:
        explanations.append("moderate semantic similarity")
    else:
        explanations.append("low semantic similarity")
    
    if credibility_score > 0.7:
        explanations.append("high source credibility")
    elif credibility_score > 0.4:
        explanations.append("moderate source credibility")
    else:
        explanations.append("low source credibility")
    
    return ", ".join(explanations)
