const { BROWSER_HEADERS } = require('../utils/browserHeaders');
const httpClient = require('../utils/httpClient');
const { cleanYouTubeDescription } = require('../utils/textCleaner');

/**
 * Helper to fetch a full video description from its page.
 */
async function fetchVideoDescription(videoId) {
    const url = `https://www.youtube.com/watch?v=${videoId}`;
    try {
        const response = await httpClient.get(url, {
            headers: { ...BROWSER_HEADERS, 'Accept-Language': 'en-US' },
            timeout: 15000
        });
        const html = response.data;
        
        let description = "";

        // 1. Try ytInitialPlayerResponse
        const playerMatch = html.match(/ytInitialPlayerResponse\s*=\s*({[\s\S]*?});/);
        if (playerMatch) {
            try {
                const data = JSON.parse(playerMatch[1]);
                description = data.videoDetails?.shortDescription || "";
            } catch (e) {}
        }

        // 2. Try ytInitialData
        if (!description) {
            const dataMatch = html.match(/ytInitialData\s*=\s*({[\s\S]*?});/);
            if (dataMatch) {
                try {
                    const data = JSON.parse(dataMatch[1]);
                    const secondary = data.contents?.twoColumnWatchNextResults?.results?.results?.contents;
                    if (secondary) {
                        const videoSecondaryInfo = secondary.find(c => c.videoSecondaryInfoRenderer)?.videoSecondaryInfoRenderer;
                        if (videoSecondaryInfo?.description) {
                            description = videoSecondaryInfo.description.runs?.map(r => r.text).join('') || videoSecondaryInfo.description.simpleText || "";
                        }
                    }
                } catch (e) {}
            }
        }

        // 3. Raw search fallback
        if (!description) {
            const rawDescMatch = html.match(/"shortDescription":"([\s\S]*?)(?<!\\)"/);
            if (rawDescMatch) {
                description = rawDescMatch[1]
                    .replace(/\\n/g, '\n')
                    .replace(/\\"/g, '"')
                    .replace(/\\u([0-9a-fA-F]{4})/g, (match, grp) => String.fromCharCode(parseInt(grp, 16)));
            }
        }

        // 4. Meta tag fallback
        if (!description) {
            const metaMatch = html.match(/<meta\s+name="description"\s+content="([^"]*)"/i) || html.match(/<meta\s+property="og:description"\s+content="([^"]*)"/i);
            if (metaMatch) description = metaMatch[1];
        }

        return description;
    } catch (error) {
        console.error(`[YouTube/Scraper] Failed to fetch description for ${videoId}: ${error.message}`);
        return "";
    }
}

/**
 * Search YouTube for videos and extract metadata.
 * Uses the ytInitialData scraping method.
 * 
 * @param {string} query Search query
 * @param {number} limit Number of results to return
 * @returns {Promise<Array>} List of video metadata objects
 */
async function searchYoutube(query, limit = 5) {
    console.log(`[YouTube] Searching for: "${query}"`);
    
    const url = `https://www.youtube.com/results?search_query=${encodeURIComponent(query)}`;
    
    try {
        const response = await httpClient.get(url, {
            headers: BROWSER_HEADERS
        });

        if (!response || typeof response.data !== 'string') {
            throw new Error('YouTube returned no HTML');
        }

        const html = response.data;
        const match = html.match(/var ytInitialData = ({.*?});/);
        
        if (!match) {
            console.warn('[YouTube] Could not find ytInitialData in HTML');
            return [];
        }

        const data = JSON.parse(match[1]);
        
        // Navigate through the complex YouTube JSON structure
        const sectionList = data.contents?.twoColumnSearchResultsRenderer?.primaryContents?.sectionListRenderer?.contents;
        if (!sectionList || !Array.isArray(sectionList)) {
            return [];
        }

        // Find the main itemSectionRenderer which contains the search results
        const itemSection = sectionList.find(c => c.itemSectionRenderer)?.itemSectionRenderer?.contents;
        if (!itemSection || !Array.isArray(itemSection)) {
            return [];
        }

        const searchResults = [];
        for (const item of itemSection) {
            if (searchResults.length >= limit) break;

            const video = item.videoRenderer;
            if (!video) continue;

            const videoId = video.videoId;
            const title = video.title?.runs?.[0]?.text || video.title?.simpleText || 'Unknown Title';
            const snippet = video.descriptionSnippet?.runs?.map(r => r.text).join('') || video.descriptionSnippet?.simpleText || '';
            const detailedSnippet = video.detailedMetadataSnippets?.[0]?.snippetText?.runs?.map(r => r.text).join('') || video.detailedMetadataSnippets?.[0]?.snippetText?.simpleText || '';
            const fallbackDescription = snippet || detailedSnippet || '';
            
            searchResults.push({
                videoId,
                title,
                url: `https://www.youtube.com/watch?v=${videoId}`,
                snippet: fallbackDescription,
                duration: video.lengthText?.simpleText || '0:00',
                views: video.shortViewCountText?.simpleText || video.viewCountText?.simpleText || '0 views',
                publishedTime: video.publishedTimeText?.simpleText || 'Unknown date',
            });
        }

        // Fetch full descriptions in parallel locally
        console.log(`[YouTube] Fetching full descriptions for up to 3 videos...`);
        const finalResults = await Promise.all(searchResults.map(async (res, index) => {
            const fullDescriptionRaw = index < 3 ? await fetchVideoDescription(res.videoId) : null;
            const description = cleanYouTubeDescription(fullDescriptionRaw || res.snippet);
            
            return {
                query: query,
                source: 'youtube',
                title: res.title,
                url: res.url,
                content: `${res.title}. ${description}`,
                score: 0.8,
                wordCount: description.split(/\s+/).length,
                description: description,
                metadata: {
                    videoId: res.videoId,
                    duration: res.duration,
                    views: res.views,
                    publishedTime: res.publishedTime,
                    is_full_description: !!fullDescriptionRaw
                }
            };
        }));

        console.log(`[YouTube] Returning ${finalResults.length} results`);
        return finalResults;
    } catch (error) {
        console.error(`[YouTube] Search failed: ${error.message}`);
        return [];
    }
}

module.exports = { searchYoutube };
