const { fetchUrl } = require('./fetcher');
const { extractReadableContent } = require('./parser');

// Import Service Scrapers
// const { scrapeArxivUrl } = require('../services/arxiv');
// const { scrapePubMedUrl } = require('../services/pubmed');
// const { scrapeGithubUrl } = require('../services/github');
// const { scrapeWikiUrl } = require('../services/wiki');

/**
 * Main Scraper / Dispatcher function
 * Routes the URL to the appropriate specialized scraper or falls back to generic scraping.
 * @param {string} url - The URL to scrape
 * @param {object} options - Options for scraping
 */
async function scrape(url, options = {}) {
  console.log(`[Dispatcher] Processing: ${url}`);

  try {
    // 1. Check for specific services
    // if (url.includes('arxiv.org')) {
    //   return await scrapeArxivUrl(url);
    // }

    // PubMed (ncbi.nlm.nih.gov/pubmed or pubmed.ncbi.nlm.nih.gov)
    // if (url.includes('pubmed.ncbi.nlm.nih.gov') || url.includes('/pubmed/')) {
    //   return await scrapePubMedUrl(url);
    // }

    // if (url.includes('github.com')) {
    //   return await scrapeGithubUrl(url);
    // }

    // if (url.includes('wikipedia.org')) {
    //   return await scrapeWikiUrl(url);
    // }

    // 2. Fallback to Generic Scraper
    console.log(
      `[Dispatcher] No specialized scraper found. Using generic scraper.`,
    );
    const response = await fetchUrl(url, options);
    console.log(
      `[Generic] Fetched ${response.html.length} bytes. Processing...`,
    );

    const result = await extractReadableContent(
      response.html,
      url,
      options.format || 'markdown',
      options,
    );

    return {
      source: 'generic',
      title: result.title,
      content: result.text,
      originalUrl: url,
      wordCount: result.text ? result.text.split(/\s+/).length : 0,
    };
  } catch (error) {
    console.error(`[Scrape Error] Failed to scrape ${url}: ${error.message}`);
    throw error;
  }
}

// Example usage if run directly
if (require.main === module) {
  const args = process.argv.slice(2);
  const targetUrl = args[0];

  if (!targetUrl) {
    console.log('Usage: node core.js <url>');
    process.exit(1);
  }

  scrape(targetUrl)
    .then((result) => {
      console.log('\n--- RESULT ---');
      console.log('Source:', result.source);
      console.log('Title:', result.title);
      console.log(
        'Content (Excerpt):',
        result.content
          ? result.content.substring(0, 200) + '...'
          : 'No content',
      );
    })
    .catch((err) => {
      console.error(err);
      process.exit(1);
    });
}

module.exports = scrape;
