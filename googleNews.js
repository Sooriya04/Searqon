const axios = require("axios");
const xml2js = require("xml2js");
const cheerio = require("cheerio");


const MAX_RESULTS = 1;


async function scrapeGoogleNewsWithContent(query) {
if (!query) throw new Error("Query is required");


const rssUrl = `https://news.google.com/rss/search?q=${encodeURIComponent(
query
)}&hl=en-IN&gl=IN&ceid=IN:en`;


const rssResponse = await axios.get(rssUrl, {
timeout: 8000,
headers: {
"User-Agent":
"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0",
},
});


const parsed = await new xml2js.Parser({ explicitArray: false })
.parseStringPromise(rssResponse.data);


const items = parsed?.rss?.channel?.item || [];
const results = [];


for (const item of items) {
if (results.length === MAX_RESULTS) break;


let url = null;
let content = "";


try {
// Extract article URL
const $desc = cheerio.load(item.description || "");
url = $desc("a").first().attr("href") || item.link;


if (!url) throw new Error("No URL");


// Fetch article page
const articleRes = await axios.get(url, {
timeout: 6000,
headers: {
"User-Agent":
"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/120.0",
},
});


const $ = cheerio.load(articleRes.data);


// Remove noise
$("script, style, nav, footer, header, aside").remove();


content =
$("article").text().trim() ||
$("main").text().trim() ||
$("p")
.map((_, el) => $(el).text())
.get()
.join(" ")
.trim();


// Accept only real article text
if (content && content.length > 300) {
results.push({
source: "google_news",
url: articleRes.request?.res?.responseUrl || url,
title: item.title || "",
content: content.replace(/\s+/g, " "),
publishedAt: item.pubDate
? new Date(item.pubDate).toISOString()
: null,
});
continue;
}
} catch {
// fall through to RSS fallback
}


// ---------- CLEAN RSS FALLBACK ----------
const rawFallback = cheerio
.load(item.description || "")
.text()
.replace(/\s+/g, " ")
.trim();


// Remove title from fallback to avoid duplication
let cleanedFallback = rawFallback
.replace(item.title || "", "")
.replace(
/(The Economic Times|Reuters|Bloomberg|American Banker|CNBC|Forbes)/gi,
""
)
.trim();


// If fallback is still useless, keep content empty
if (cleanedFallback.length < 80) {
cleanedFallback = "";
}


results.push({
source: "google_news",
url,
title: item.title || "",
content: cleanedFallback,
publishedAt: item.pubDate
? new Date(item.pubDate).toISOString()
: null,
});
}


return results;
}
// ----------------- RUN -----------------
(async () => {
const data = await scrapeGoogleNewsWithContent("Cladue AI");
console.log(data);
})();