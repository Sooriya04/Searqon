const cheerio = require("cheerio");
const httpClient = require("./httpClient");
const { BROWSER_HEADERS } = require("./browserHeaders");
const { cleanText, removePageMetadataNoise } = require("./textCleaner");

const CONTENT_SELECTORS = [
  "article",
  "[role='main']",
  "main",
  ".post-content",
  ".article-content",
  ".entry-content",
  ".content-body",
  ".article-body",
  ".story-body",
  "#article-body",
  ".post-body",
  ".blog-post",
  ".single-post",
  "#content",
  ".content",
  ".main-content",
];

const NOISE_SELECTORS = [
  "script",
  "style",
  "noscript",
  "iframe",
  "svg",
  "canvas",
  "nav",
  "footer",
  "header",
  "aside",
  ".nav",
  ".menu",
  ".sidebar",
  ".footer",
  ".header",
  ".navigation",
  ".ads",
  ".advertisement",
  ".ad",
  ".advert",
  ".sponsored",
  ".social",
  ".share",
  ".sharing",
  ".social-share",
  ".comments",
  ".comment",
  "#comments",
  ".comment-section",
  ".related",
  ".recommended",
  ".more-stories",
  "form",
  "button",
  "input",
  "select",
  "textarea",
  "[role='navigation']",
  "[role='banner']",
  "[role='complementary']",
];


async function fetchHtml(url) {
  try {
    const response = await httpClient.get(url, {
      headers: BROWSER_HEADERS,
      responseType: "text",
      timeout: 15000,
      validateStatus: (status) => status >= 200 && status < 400,
    });

    if (!response || typeof response.data !== "string") {
      return null;
    }

    return response.data;
  } catch {
    return null;
  }
}

function extractMetadata($) {
  return {
    title: $("title").text().trim() || $("h1").first().text().trim(),
    description: $('meta[name="description"]').attr("content") || "",
    author:
      $('meta[name="author"]').attr("content") ||
      $('[rel="author"]').text().trim() ||
      $(".author").first().text().trim() ||
      "",
    publishedDate:
      $('meta[property="article:published_time"]').attr("content") ||
      $("time[datetime]").attr("datetime") ||
      $(".date, .published, .post-date").first().text().trim() ||
      "",
  };
}

function removeNoiseElements($) {
  NOISE_SELECTORS.forEach((selector) => {
    $(selector).remove();
  });
}

function findContentArea($) {
  for (const selector of CONTENT_SELECTORS) {
    const found = $(selector).first();
    if (found.length && found.text().trim().length > 200) {
      return found;
    }
  }
  return $("body");
}

function extractParagraphs($, contentArea) {
  const paragraphs = [];
  contentArea.find("figure, img, picture, video, audio").remove();
  contentArea.find(".caption, figcaption").remove();
  contentArea.find("hr, br").replaceWith("\n");

  contentArea
    .find("p, h1, h2, h3, h4, h5, h6, li, blockquote, pre, td, th")
    .each((i, el) => {
      const text = $(el).text().trim();
      if (text && text.length > 20) {
        const tagName = el.tagName?.toLowerCase() || el.name?.toLowerCase();
        if (tagName && tagName.match(/^h[1-6]$/)) {
          paragraphs.push(`\n## ${text}\n`);
        } else {
          paragraphs.push(text);
        }
      }
    });

  return paragraphs;
}
function calculateQualityScore(content) {
  const wordCount = content.split(/\s+/).length;
  const hasGoodLength = wordCount > 100;
  const hasStructure = content.includes("\n");

  if (hasGoodLength && hasStructure) return 0.9;
  if (hasGoodLength) return 0.7;
  return 0.5;
}

async function extractPageContent(url) {
  const html = await fetchHtml(url);
  if (!html) return null;

  const $ = cheerio.load(html);


  const metadata = extractMetadata($);

  removeNoiseElements($);

  const contentArea = findContentArea($);

  const paragraphs = extractParagraphs($, contentArea);

  let rawContent =
    paragraphs.length > 0 ? paragraphs.join("\n\n") : contentArea.text();

  let content = cleanText(rawContent);
  content = removePageMetadataNoise(content);

  const wordCount = content.split(/\s+/).length;
  const score = calculateQualityScore(content);

  return {
    title: cleanText(metadata.title),
    content,
    author: cleanText(metadata.author),
    publishedDate: metadata.publishedDate,
    metaDescription: cleanText(metadata.description),
    wordCount,
    score,
    isQualityContent: score >= 0.7,
  };
}

module.exports = {
  extractPageContent,
  fetchHtml,
  extractMetadata,
  calculateQualityScore,
};
