// const {fetch} = require("node-fetch");
const REDDIT_SEARCH_URL = "https://www.reddit.com/search.json";

async function reddit(query) {
    const url = `${REDDIT_SEARCH_URL}?q=${encodeURIComponent(query)}`;
    const response = await fetch(url, {
        headers : {
            "User-Agent" : "SearqonBot/1.0"
        }
    });

    if(!response.ok){
        throw new Error("Failed to fetch reddit data");
    }
    const data = await response.json();

    return data.data.children.map(post => ({
        title: post.data.title,
        subreddit: post.data.subreddit,
        score: post.data.score,
        url: `https://reddit.com${post.data.permalink}`,
        created_utc: post.data.created_utc,
        content: post.data.selftext || ""
    }));
}

module.exports = {
    reddit
}