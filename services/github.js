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
    return data.items || [];
}

async function searchWithReadmes(query, limit = 10) {
    console.log(`[GitHub] Searching for: "${query}"`);

    try {
        const repos = await githubSearch(query, limit);
        const savedResults = await Promise.all(repos.map(async (repo, index) => {
            let content = '';
            try {
                if (index < 3) {
                    const scraped = await ScrapUrl(repo.html_url);
                    if (scraped && scraped.content) {
                        content = scraped.content;
                    }
                }
            } catch (e) { }

            if (!content || content.length < 20) {
                content = repo.description || '';
            }

            if (content.length >= 5 || repo.description) {
                return {
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
                };
            }
            return null;
        }));

        const filteredResults = savedResults.filter(r => r !== null);

        console.log(`[GitHub] Returning ${filteredResults.length} results`);
        return filteredResults;
    } catch (error) {
        console.error(`[GitHub] Search failed: ${error.message}`);
        return [];
    }
}

module.exports = {
    searchWithReadmes,
};
