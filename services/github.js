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

async function searchWithReadmes(query) {
  const repos = await githubSearch(query, 10);

  const results = await Promise.all(
    repos.map(async repo => {
      const readme = await fetchReadme(repo.owner.login, repo.name);

      return {
        name: repo.full_name,
        description: repo.description || "",
        stars: repo.stargazers_count,
        forks: repo.forks_count,
        language: repo.language,
        url: repo.html_url,
        readme: readme || ""
      };
    })
  );

  return results;
}

module.exports = {
  searchWithReadmes
};
