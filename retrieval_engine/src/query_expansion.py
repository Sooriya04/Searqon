from typing import List, Set

class QueryExpander:
    def __init__(self, use_wordnet: bool = False):
        self.use_wordnet = use_wordnet
        self.manual_synonyms = self._get_manual_synonyms()
    
    def _get_manual_synonyms(self) -> dict:
        return {
            'ai': ['artificial intelligence', 'machine learning'],
            'machine learning': ['ml', 'deep learning'],
            'health': ['medical', 'healthcare'],
            'technology': ['tech', 'computing'],
            'education': ['learning', 'school'],
            'security': ['cybersecurity', 'privacy'],
        }
    
    def expand_query(self, query: str, max_expansion: int = 3) -> str:
        query_lower = query.lower()
        expansion_terms = []
        
        for term, synonyms in self.manual_synonyms.items():
            if term in query_lower:
                expansion_terms.extend(synonyms[:max_expansion])
        
        expansion_terms = list(set(expansion_terms))
        expansion_terms = [t for t in expansion_terms if t not in query_lower]
        expansion_terms = expansion_terms[:max_expansion]
        
        expanded_query = query + " " + " ".join(expansion_terms)
        return expanded_query.strip()
