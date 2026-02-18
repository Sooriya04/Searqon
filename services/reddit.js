const { BROWSER_HEADERS } = require('../utils/browserHeaders');
const ScrapUrl = require('../scrapper/ScrapUrl');
const { cleanText } = require('../utils/textCleaner');

const REDDIT_SEARCH_URL = 'https://www.reddit.com/search.json';

async function reddit(query, limit = 10) {
    console.log(`[Reddit] Searching for: "${query}"`);

    const url = `${REDDIT_SEARCH_URL}?q=${encodeURIComponent(query)}&limit=${limit}`;

    try {
        const response = await fetch(url, {
            headers: {
                ...BROWSER_HEADERS,
                // Reddit API requires a specific UA format or it blocks generic ones
                'User-Agent':
                    'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36 Searqon/1.0',
            },
        });

        if (!response.ok) {
            throw new Error('Failed to fetch reddit data');
        }
        const data = await response.json();

        const savedResults = [];

        if (data.data && data.data.children) {
            for (const post of data.data.children) {
                const postData = post.data;
                const resultTitle = cleanText(postData.title);
                const rawContent = postData.selftext || postData.title || '';
                let content = cleanText(rawContent);

                const isExternalLink =
                    !postData.is_self && postData.url && !postData.url.includes('reddit.com');

                // If it's an external link, try to scrape it (Always)
                if (isExternalLink) {
                    try {
                        const scraped = await ScrapUrl(postData.url);
                        if (scraped && scraped.content) {
                            content += '\n\n' + cleanText(scraped.content);
                        }
                    } catch (e) {
                        // Ignore scraping errors
                    }
                }

                // Only save posts with meaningful content
                if (content.length >= 10) {
                    const resultData = {
                        query: query,
                        source: 'reddit',
                        title: resultTitle,
                        url: `https://reddit.com${postData.permalink}`,
                        content: content,
                        score: (postData.score || 0) / 1000,
                        wordCount: content.split(/\s+/).length,
                        author: postData.author || 'unknown',
                        publishedDate: new Date(postData.created_utc * 1000).toISOString(),
                        body_html: postData.selftext_html || '',
                        metadata: {
                            subreddit: postData.subreddit,
                            upvotes: postData.ups,
                            comments: postData.num_comments,
                            awards: postData.total_awards_received || 0,
                        },
                    };
                    savedResults.push({
                        query: resultData.query,
                        source: resultData.source,
                        title: resultData.title,
                        url: resultData.url,
                        content: resultData.content, // Includes selftext + scraped external content
                        body_html: resultData.body_html,
                        subreddit: postData.subreddit,
                        score: resultData.score,
                        author: resultData.author,
                        wordCount: resultData.wordCount,
                    });
                }
            }
        }

        console.log(`[Reddit] Returning ${savedResults.length} results`);
        return savedResults;
    } catch (error) {
        console.error(`[Reddit] Search failed: ${error.message}`);
        return [];
    }
}

module.exports = {
    reddit,
};
