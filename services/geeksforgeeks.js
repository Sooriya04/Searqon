const { searchDuckDuckGo } = require('./duckduckgo');


async function searchGeeksForGeeks(query, limit = 5) {
    // Try a more natural query first as site: operator can sometimes yield 0 results on DDG HTML
    const searchString = `${query} geeksforgeeks`;
    console.log(`[GeeksForGeeks] Searching via DDG: "${searchString}"`);

    let results = await searchDuckDuckGo(searchString, limit + 2); // Fetch extra to account for filtering
    console.log(`[GeeksForGeeks] DDG returned ${results.length} results (pre-filter)`);

    // Filter for strictly geeksforgeeks.org URLs
    results = results.filter(r => r.url && r.url.includes('geeksforgeeks.org'));

    // If still 0, try specific site operator as fallback (though unlikely to help if above failed)
    if (results.length === 0) {
        const siteQuery = `site:geeksforgeeks.org ${query}`;
        results = await searchDuckDuckGo(siteQuery, limit);
    }

    // Remap the source to 'geeksforgeeks', clean content, and restore original query
    const mapped = results.slice(0, limit).map(result => ({
        ...result,
        source: 'geeksforgeeks',
        query: query,
        content: cleanGFGContent(result.content)
    }));
    console.log(`[GeeksForGeeks] Returning ${mapped.length} mapped results`);
    return mapped;
}

function cleanGFGContent(text) {
    if (!text) return "";

    let cleaned = text;

    const lastUpdatedIndex = cleaned.indexOf("Last Updated :");
    if (lastUpdatedIndex !== -1) {
        cleaned = cleaned.substring(lastUpdatedIndex);
    } else {
        const weightageIndex = cleaned.indexOf("Weightage Analysis");
        if (weightageIndex !== -1) {
            cleaned = cleaned.substring(weightageIndex);
        } else {
            if (cleaned.startsWith("Courses")) {
                const practiceIndex = cleaned.indexOf("Practice");
                if (practiceIndex !== -1 && practiceIndex < 50) { // Ensure it's at the start
                    const doubleNewline = cleaned.indexOf("\n\n", practiceIndex);
                    if (doubleNewline !== -1) {
                        cleaned = cleaned.substring(doubleNewline).trim();
                    }
                }
            }
        }
    }

    const footerMarkers = [
        "Suggested Quiz",
        "Article Tags:",
        "Improve Article",
        "Please Login to comment",
        "@GeeksforGeeks, Sanchhaya Education"
    ];

    let cutOffIndex = cleaned.length;
    for (const marker of footerMarkers) {
        const index = cleaned.indexOf(marker);
        if (index !== -1 && index < cutOffIndex) {
            cutOffIndex = index;
        }
    }
    cleaned = cleaned.substring(0, cutOffIndex);


    // Remove other specific noise lines (exact matches or simple replacements)
    const specificNoise = [
        "Switch to Dark Mode",
        "Elevate your practice",
        "My Personal Notes",
        "Sign In",
    ];

    specificNoise.forEach(noise => {
        if (cleaned.includes(noise)) {
            cleaned = cleaned.split(noise).join("");
        }
    });

    return cleaned.trim();
}

module.exports = { searchGeeksForGeeks };
