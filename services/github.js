const ScrapUrl = require('../scrapper/ScrapUrl');

const GITHUB_SEARCH_URL = 'https://api.github.com/search/repositories';

async function githubSearch(query, limit = 10) {
  const url = `${GITHUB_SEARCH_URL}?q=${encodeURIComponent(query)}&per_page=${limit}`;

  const response = await fetch(url, {
    headers: {
      'User-Agent': 'SearqonBot/1.0',
      Accept: 'application/vnd.github+json',
    },
  });

  if (!response.ok) {
    throw new Error('Failed to fetch GitHub repositories');
  }

  const data = await response.json();
  return data.items;
}

async function searchWithReadmes(query, limit = 10) {
  console.log(`[GitHub] Searching for: "${query}"`);

  const repos = await githubSearch(query, limit);
  const savedResults = [];

  for (const repo of repos) {
    let content = '';

    // Scrape the repo page for rich content (README rendered + about)
    try {
      console.log(`[GitHub] Scraping: ${repo.html_url}`);
      const scraped = await ScrapUrl(repo.html_url);
      if (scraped && scraped.content) {
        content = scraped.content;
      }
    } catch (e) {
      console.log(`[GitHub] Scraping failed for ${repo.html_url}: ${e.message}`);
    }

    // Fallback to description if scraping failed
    if (!content || content.length < 20) {
      content = repo.description || '';
    }

    if (content.length >= 20) {
      savedResults.push({
        query: query,
        source: 'github',
        name: repo.full_name,
        title: repo.full_name,
        url: repo.html_url,
        content: content,
        description: repo.description || '',
        stars: repo.stargazers_count,
        forks: repo.forks_count,
        language: repo.language,
        score: Math.min(repo.stargazers_count / 10000, 1),
        wordCount: content.split(/\s+/).length,
        author: repo.owner.login,
      });
    }
  }

  console.log(`[GitHub] Returning ${savedResults.length} results`);
  return savedResults;
}

module.exports = {
  searchWithReadmes,
};
