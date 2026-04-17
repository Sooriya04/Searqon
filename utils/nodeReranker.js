/**
 * Searqon Node.js Reranker
 * 
 * A high-performance, zero-RAM implementation of BM25 and Credibility scoring.
 * Minimal port of the Python Retrieval Engine logic.
 */

const TRUSTED_SOURCES = new Set([
  'nature', 'science', 'ieee', 'acm', 'arxiv',
  'medical', 'journal', 'research', 'university', 'college',
  'nyt', 'reuters', 'bbc', 'associated press', 'ap news'
]);

const CLICKBAIT_KEYWORDS = [
  'shocking', 'unbelievable', 'doctors hate', 'secret', 'truth bomb',
  'leaked', 'exclusive', 'conspiracy', 'they dont want', 'proof inside',
  'miracle', 'cure', 'wake up', 'sheeple', 'hidden', 'exposed'
];

const STOP_WORDS = new Set([
  'i', 'me', 'my', 'myself', 'we', 'our', 'ours', 'ourselves', 'you', "you're", "you've", "you'll", "you'd",
  'your', 'yours', 'yourself', 'yourselves', 'he', 'him', 'his', 'himself', 'she', "she's", 'her', 'hers',
  'herself', 'it', "it's", 'its', 'itself', 'they', 'them', 'their', 'theirs', 'themselves', 'what', 'which',
  'who', 'whom', 'this', 'that', "that'll", 'these', 'those', 'am', 'is', 'are', 'was', 'were', 'be', 'been',
  'being', 'have', 'has', 'had', 'having', 'do', 'does', 'did', 'doing', 'a', 'an', 'the', 'and', 'but', 'if',
  'or', 'because', 'as', 'until', 'while', 'of', 'at', 'by', 'for', 'with', 'about', 'against', 'between', 'into',
  'through', 'during', 'before', 'after', 'above', 'below', 'to', 'from', 'up', 'down', 'in', 'out', 'on', 'off',
  'over', 'under', 'again', 'further', 'then', 'once', 'here', 'there', 'when', 'where', 'why', 'how', 'all', 'any',
  'both', 'each', 'few', 'more', 'most', 'other', 'some', 'such', 'no', 'nor', 'not', 'only', 'own', 'same', 'so',
  'than', 'too', 'very', 's', 't', 'can', 'will', 'just', 'don', "don't", 'should', "should've", 'now'
]);

function tokenize(text) {
  if (!text) return [];
  return text.toLowerCase()
    .replace(/[^\w\s]/g, '')
    .split(/\s+/)
    .filter(t => t.length > 2 && !STOP_WORDS.has(t));
}

/**
 * Basic BM25 implementation for a small set of documents.
 * Since we have the full text for all docs, we can compute local stats instantly.
 */
function computeBM25(queryTokens, documents) {
  const k1 = 1.5;
  const b = 0.75;
  
  const docTokens = documents.map(d => tokenize(`${d.title} ${d.content}`));
  const avgdl = docTokens.reduce((acc, tokens) => acc + tokens.length, 0) / documents.length;
  
  // Calculate Inverse Document Frequency (IDF)
  const N = documents.length;
  const idfMap = {};
  queryTokens.forEach(token => {
    const nq = docTokens.filter(tokens => tokens.includes(token)).length;
    idfMap[token] = Math.log(1 + (N - nq + 0.5) / (nq + 0.5));
  });

  return docTokens.map((tokens, idx) => {
    let score = 0;
    const dl = tokens.length;
    const termFreqs = {};
    tokens.forEach(t => termFreqs[t] = (termFreqs[t] || 0) + 1);

    queryTokens.forEach(token => {
      const fq = termFreqs[token] || 0;
      const idf = idfMap[token] || 0;
      score += idf * (fq * (k1 + 1)) / (fq + k1 * (1 - b + b * (dl / avgdl)));
    });
    return score;
  });
}

/**
 * Rule-based Credibility Scorer
 */
function computeCredibility(doc) {
  const source = (doc.source || '').toLowerCase();
  const title = (doc.title || '').toLowerCase();
  const content = (doc.content || '').toLowerCase();
  
  // 1. Source Trust
  let sourceScore = 0.5;
  for (let trusted of TRUSTED_SOURCES) {
    if (source.includes(trusted)) { sourceScore = 0.9; break; }
  }
  if (source.includes('blog') || source.includes('gossip') || source.includes('viral')) {
    sourceScore = 0.2;
  }

  // 2. Clickbait Detection
  let clickbaitCount = 0;
  for (let kw of CLICKBAIT_KEYWORDS) {
    if (title.includes(kw) || content.includes(kw)) clickbaitCount++;
  }
  const clickbaitScore = Math.max(0.3, 1.0 - (clickbaitCount * 0.15));

  // 3. Readability / Quality
  const wordCount = content.split(/\s+/).length;
  let lengthScore = 0.3;
  if (wordCount > 500) lengthScore = 1.0;
  else if (wordCount > 200) lengthScore = 0.85;
  else if (wordCount > 50) lengthScore = 0.6;

  // Final Fusion (same weights as Python)
  const overall = (0.4 * sourceScore + 0.25 * clickbaitScore + 0.35 * lengthScore);
  return overall;
}

/**
 * Main Reranking Logic for Node.js
 */
function nodeRerank(query, documents, limit = 10) {
  const queryTokens = tokenize(query);
  if (queryTokens.length === 0 || documents.length === 0) return documents.slice(0, limit);

  // 1. Lexical BM25
  const bm25Scores = computeBM25(queryTokens, documents);
  const maxBM25 = Math.max(...bm25Scores) || 1.0;

  // 2. Credibility
  const credScores = documents.map(computeCredibility);

  // 3. Hybrid Fusion & Explanation
  const results = documents.map((doc, idx) => {
    const bm25Norm = bm25Scores[idx] / maxBM25;
    const credScore = credScores[idx];
    
    // Node.js weights: 70% relevance, 30% credibility
    const finalScore = (0.7 * bm25Norm + 0.3 * credScore);
    
    let explanation = "Strong keyword match";
    if (credScore > 0.8) explanation += " from a highly trusted source";
    if (bm25Norm < 0.3) explanation = "Relevant source but low direct keyword density";

    return {
      ...doc,
      score: Math.round(finalScore * 1000) / 1000,
      explanation: explanation,
      metadata: {
        ...(doc.metadata || {}),
        ranker: 'node_lite',
        bm25_norm: Math.round(bm25Norm * 100) / 100,
        cred_score: Math.round(credScore * 100) / 100
      }
    };
  });

  return results
    .sort((a, b) => b.score - a.score)
    .slice(0, limit);
}

module.exports = { nodeRerank };
