import re
from typing import Dict, Tuple

class CredibilityScorer:
    TRUSTED_SOURCES = {
        'nature', 'science', 'ieee', 'acm', 'arxiv',
        'medical', 'journal', 'research', 'university', 'college',
        'nyt', 'reuters', 'bbc', 'associated press', 'ap news',
        'review', 'quarterly', 'digest', 'today', 'news'
    }
    
    CLICKBAIT_KEYWORDS = {
        'shocking', 'unbelievable', 'doctors hate', 'secret', 'truth bomb',
        'leaked', 'exclusive', 'conspiracy', 'they dont want', 'proof inside',
        'miracle', 'cure', 'wake up', 'sheeple', 'hidden', 'exposed',
        'banned', 'suppressed', 'one trick', 'hate', 'unverified'
    }
    
    NEGATIVE_SOURCE_INDICATORS = {
        'blog', 'gossip', 'viral', 'conspiracy', 'truth', 'exposed',
        'unverified', 'alternative', 'scam'
    }
    
    def _source_credibility_score(self, source: str) -> float:
        source_lower = source.lower()
        for indicator in self.NEGATIVE_SOURCE_INDICATORS:
            if indicator in source_lower:
                return 0.2
        for trusted in self.TRUSTED_SOURCES:
            if trusted in source_lower:
                return 0.9
        return 0.5
    
    def _clickbait_score(self, title: str, content: str) -> float:
        combined_text = (str(title) + " " + str(content)).lower()
        clickbait_count = sum(1 for keyword in self.CLICKBAIT_KEYWORDS if keyword in combined_text)
        penalty = clickbait_count * 0.15
        return max(0.3, 1.0 - penalty)
    
    def _readability_score(self, content: str) -> float:
        content = str(content)
        word_count = len(content.split())
        if word_count < 50:
            length_score = 0.3
        elif word_count < 200:
            length_score = 0.6
        elif word_count < 500:
            length_score = 0.85
        else:
            length_score = 1.0
        
        url_count = len(re.findall(r'http[s]?://', content))
        citation_keywords = content.count('cite') + content.count('reference')
        citation_score = min(1.0, (url_count + citation_keywords) * 0.1 + 0.5)
        
        return 0.7 * length_score + 0.3 * citation_score
    
    def _title_content_consistency(self, title: str, content: str) -> float:
        title_words = set(w.lower() for w in str(title).split() if len(w) > 3)
        content_words = set(w.lower() for w in str(content).split() if len(w) > 3)
        
        if not title_words:
            return 0.5
        
        overlap = len(title_words & content_words) / len(title_words)
        
        if overlap < 0.1:
            return 0.4
        elif overlap < 0.3:
            return 0.6
        else:
            return 0.85
    
    def compute_credibility(self, source: str, title: str, content: str,
                          credibility_label: str = None) -> Tuple[float, Dict[str, float]]:
        if credibility_label and str(credibility_label).lower() == 'trusted':
            return 0.9, {'source': 0.9, 'clickbait': 0.9, 'readability': 0.85, 'consistency': 0.85}
        elif credibility_label and str(credibility_label).lower() == 'low':
            return 0.25, {'source': 0.2, 'clickbait': 0.3, 'readability': 0.4, 'consistency': 0.5}
        
        source_score = self._source_credibility_score(str(source))
        clickbait_score = self._clickbait_score(title, content)
        readability_score = self._readability_score(content)
        consistency_score = self._title_content_consistency(title, content)
        
        component_scores = {'source': source_score, 'clickbait': clickbait_score,
                          'readability': readability_score, 'consistency': consistency_score}
        
        overall_score = (0.4 * source_score + 0.25 * clickbait_score + 
                        0.2 * readability_score + 0.15 * consistency_score)
        
        return overall_score, component_scores
    
    def score_documents(self, documents: Dict[str, Dict]) -> Dict[str, float]:
        scores = {}
        for doc_id, doc in documents.items():
            source = doc.get('source', 'Unknown')
            title = doc.get('title', '')
            content = doc.get('content', '')
            label = doc.get('credibility_label', None)
            score, _ = self.compute_credibility(source, title, content, label)
            scores[doc_id] = score
        return scores
