const DatabaseClient = require("../database/client");

const REDDIT_SEARCH_URL = "https://www.reddit.com/search.json";

async function reddit(query, limit = 10) {
    console.log(`[Reddit] Searching for: "${query}"`);
    
    const url = `${REDDIT_SEARCH_URL}?q=${encodeURIComponent(query)}&limit=${limit}`;
    const response = await fetch(url, {
        headers : {
            "User-Agent" : "SearqonBot/1.0"
        }
    });

    if(!response.ok){
        throw new Error("Failed to fetch reddit data");
    }
    const data = await response.json();

    const savedResults = [];

    for (const post of data.data.children) {
        const postData = post.data;
        const content = postData.selftext || postData.title || "";
        
        // Only save posts with meaningful content
        if (content.length >= 20) {
            const resultData = {
                query: query,
                source: "reddit",
                title: postData.title,
                url: `https://reddit.com${postData.permalink}`,
                content: content,
                score: (postData.score || 0) / 1000, // Normalize score to 0-1 range
                wordCount: content.split(/\s+/).length,
                author: postData.author || "unknown",
                publishedDate: new Date(postData.created_utc * 1000).toISOString(),
                metadata: {
                    subreddit: postData.subreddit,
                    upvotes: postData.ups,
                    comments: postData.num_comments,
                    awards: postData.total_awards_received || 0
                }
            };

            // Save to database microservice
            await DatabaseClient.saveResult(resultData);
            console.log(`[Reddit] Saved result to database: ${postData.title}`);

            savedResults.push({
                query: resultData.query,
                source: resultData.source,
                title: resultData.title,
                url: resultData.url,
                content: resultData.content,
                subreddit: postData.subreddit,
                score: resultData.score,
                author: resultData.author,
                wordCount: resultData.wordCount
            });
        }
    }

    console.log(`[Reddit] Returning ${savedResults.length} results`);
    return savedResults;
}

module.exports = {
    reddit
}