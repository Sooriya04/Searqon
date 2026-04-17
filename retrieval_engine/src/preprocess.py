import re
import string
from typing import List

# Simple fallback stopwords set without needing NLTK downloads
STOP_WORDS = {
    'i', 'me', 'my', 'myself', 'we', 'our', 'ours', 'ourselves', 'you', "you're", "you've", "you'll", "you'd",
    'your', 'yours', 'yourself', 'yourselves', 'he', 'him', 'his', 'himself', 'she', "she's", 'her', 'hers',
    'herself', 'it', "it's", 'its', 'itself', 'they', 'them', 'their', 'theirs', 'themselves', 'what', 'which',
    'who', 'whom', 'this', 'that', "that'll", 'these', 'those', 'am', 'is', 'are', 'was', 'were', 'be', 'been',
    'being', 'have', 'has', 'had', 'having', 'do', 'does', 'did', 'doing', 'a', 'an', 'the', 'and', 'but', 'if',
    'or', 'because', 'as', 'until', 'while', 'of', 'at', 'by', 'for', 'with', 'about', 'against', 'between', 'into',
    'through', 'during', 'before', 'after', 'above', 'below', 'to', 'from', 'up', 'down', 'in', 'out', 'on', 'off',
    'over', 'under', 'again', 'further', 'then', 'once', 'here', 'there', 'when', 'where', 'why', 'how', 'all', 'any',
    'both', 'each', 'few', 'more', 'most', 'other', 'some', 'such', 'no', 'nor', 'not', 'only', 'own', 'same', 'so',
    'than', 'too', 'very', 's', 't', 'can', 'will', 'just', 'don', "don't", 'should', "should've", 'now', 'd', 'll',
    'm', 'o', 're', 've', 'y', 'ain', 'aren', "aren't", 'couldn', "couldn't", 'didn', "didn't", 'doesn', "doesn't",
    'hadn', "hadn't", 'hasn', "hasn't", 'haven', "haven't", 'isn', "isn't", 'ma', 'mightn', "mightn't", 'mustn',
    "mustn't", 'needn', "needn't", 'shan', "shan't", 'shouldn', "shouldn't", 'wasn', "wasn't", 'weren', "weren't",
    'won', "won't", 'wouldn', "wouldn't"
}

class TextPreprocessor:
    def __init__(self, lowercase: bool = True, remove_punctuation: bool = True,
                 remove_stopwords: bool = True, lemmatize: bool = False):
        self.lowercase = lowercase
        self.remove_punctuation = remove_punctuation
        self.remove_stopwords = remove_stopwords
        self.lemmatize = lemmatize  # Disabled deliberately for speed; pure python has no easy lemmatizer
        self.stop_words = STOP_WORDS
    
    def preprocess(self, text: str) -> List[str]:
        if not isinstance(text, str):
            text = str(text) if text is not None else ""
            
        if self.lowercase:
            text = text.lower()
        
        if self.remove_punctuation:
            text = text.translate(str.maketrans('', '', string.punctuation))
            
        # Basic regex tokenization: split on non-alphanumeric
        tokens = re.findall(r'\b\w+\b', text)
        
        if self.remove_stopwords:
            tokens = [t for t in tokens if t not in self.stop_words]
            
        # Optional: very basic plural stemming if lemmatize is requested (fallback)
        if self.lemmatize:
            tokens = [t[:-1] if t.endswith('s') and not t.endswith('ss') else t for t in tokens]
        
        return tokens
    
    def preprocess_to_string(self, text: str) -> str:
        tokens = self.preprocess(text)
        return " ".join(tokens)

def preprocess_batch(texts: List[str], preprocessor: 'TextPreprocessor') -> List[List[str]]:
    return [preprocessor.preprocess(text) for text in texts]

def preprocess_batch_to_strings(texts: List[str], preprocessor: 'TextPreprocessor') -> List[str]:
    return [preprocessor.preprocess_to_string(text) for text in texts]
