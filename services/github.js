const DatabaseClient = require("../connection/client");

const GITHUB_SEARCH_URL = "https://api.github.com/search/repositories";
const GITHUB_REPO_URL = "https://api.github.com/repos";

async function githubSearch(query, limit = 10) {
  const url = `${GITHUB_SEARCH_URL}?q=${encodeURIComponent(query)}&per_page=${limit}`;

  const response = await fetch(url, {
    headers: {
      "User-Agent": "SearqonBot/1.0",
      "Accept": "application/vnd.github+json"
    }
  });

  if (!response.ok) {
    throw new Error("Failed to fetch GitHub repositories");
  }

  const data = await response.json();
  return data.items;
}

async function fetchReadme(owner, repo) {
  const url = `${GITHUB_REPO_URL}/${owner}/${repo}/readme`;

  const response = await fetch(url, {
    headers: {
      "User-Agent": "SearqonBot/1.0",
      "Accept": "application/vnd.github.raw"
    }
  });

  if (!response.ok) {
    return null; // README may not exist
  }

  return response.text();
}

async function searchWithReadmes(query, limit = 10) {
  console.log(`[GitHub] Searching for: "${query}"`);
  
  const repos = await githubSearch(query, limit);
  const savedResults = [];

  for (const repo of repos) {
    const readme = await fetchReadme(repo.owner.login, repo.name);
    const content = readme || repo.description || "";

    // Only save repos with meaningful content
    if (content.length >= 20) {
      const resultData = {
        query: query,
        source: "github",
        title: repo.full_name,
        url: repo.html_url,
        content: content,
        score: Math.min(repo.stargazers_count / 10000, 1), // Normalize score
        wordCount: content.split(/\s+/).length,
        author: repo.owner.login,
        publishedDate: repo.created_at,
        metadata: {
          stars: repo.stargazers_count,
          forks: repo.forks_count,
          language: repo.language,
          watchers: repo.watchers_count,
          open_issues: repo.open_issues_count,
          description: repo.description || ""
        }
      };

      // Save to database microservice
      await DatabaseClient.saveResult(resultData);
      console.log(`[GitHub] Saved result to database: ${repo.full_name}`);

      savedResults.push({
        query: resultData.query,
        source: resultData.source,
        name: repo.full_name,
        title: resultData.title,
        url: resultData.url,
        description: repo.description || "",
        stars: repo.stargazers_count,
        forks: repo.forks_count,
        language: repo.language,
        readme: readme || "",
        wordCount: resultData.wordCount
      });
    }
  }

  console.log(`[GitHub] Returning ${savedResults.length} results`);
  return savedResults;
}

module.exports = {
  searchWithReadmes
};
