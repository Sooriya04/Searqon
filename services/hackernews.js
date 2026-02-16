const ScrapUrl = require('../scrapper/ScrapUrl');

async function searchHNByQuery(query, limit = 10) {
  console.log(`[HackerNews] Searching for: "${query}"`);
  const searchUrl = `https://hn.algolia.com/api/v1/search?query=${encodeURIComponent(
    query,
  )}&tags=story&hitsPerPage=${limit}`;

  const response = await fetch(searchUrl, {
    headers: {
      Accept: 'application/json',
      'User-Agent': 'SearqonBot/1.0',
    },
  });

  if (!response.ok) {
    throw new Error(`HackerNews API returned ${response.status}`);
  }

  const data = await response.json();
  const savedResults = [];

  if (data && data.hits) {
    // Fetch content for all URLs in parallel
    const contentPromises = data.hits.map(async (hit) => {
      const title = hit.title || '';
      const url =
        hit.url || `https://news.ycombinator.com/item?id=${hit.objectID}`;

      // Fetch actual page content via Scrapper
      let content = 'Content could not be fetched';
      try {
        const scrapeResult = await ScrapUrl(url);
        content = scrapeResult.content || content;
      } catch (err) {
        console.error(`[HackerNews] Failed to scrape ${url}: ${err.message}`);
      }

      if (title && content && content !== 'Content could not be fetched') {
        const resultData = {
          query: query,
          source: 'hackernews',
          title: title,
          url: url,
          content: content,
          score: Math.min((hit.points || 0) / 500, 1), // Normalize score to 0-1 range
          wordCount: content.split(/\s+/).length,
          author: hit.author || 'unknown',
          publishedDate: hit.created_at || null,
          metadata: {
            points: hit.points || 0,
            num_comments: hit.num_comments || 0,
            story_id: hit.objectID,
          },
        };
        return {
          query: resultData.query,
          source: resultData.source,
          title: resultData.title,
          url: resultData.url,
          content: resultData.content,
          points: hit.points || 0,
          author: resultData.author,
          wordCount: resultData.wordCount,
          created_at: hit.created_at || null,
        };
      }
      return null;
    });

    const fetchedResults = await Promise.all(contentPromises);
    savedResults.push(...fetchedResults.filter((r) => r !== null));
  }

  return savedResults;
}

module.exports = {
  searchHNByQuery,
};
