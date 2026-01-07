
function cleanText(text) {
  if (!text) return "";

  return text
    .replace(/[\u2018\u2019\u201A\u201B\u2032]/g, "'")
    .replace(/[\u201C\u201D\u201E\u201F\u2033]/g, '"')
    .replace(/[\u2013\u2014]/g, "-")
    .replace(/[\u2026]/g, "...")
    .replace(/[\u00AD]/g, "")
    .replace(/[\u200B-\u200D\uFEFF]/g, "")
    .replace(/\t/g, " ")
    .replace(/[ ]{2,}/g, " ")
    .replace(/\n{3,}/g, "\n\n")
    .split("\n")
    .map((line) => line.trim())
    .filter((line) => line.length > 0)
    .join("\n")
    .trim();
}

function removePageMetadataNoise(content) {
  if (!content) return "";

  return content
    .split("\n")
    .filter((line) => {
      const lowerLine = line.toLowerCase();
      if (lowerLine.startsWith("categories:")) return false;
      if (lowerLine.match(/^(articles?|pages?) (with|using|including)/i)) return false;
      if (lowerLine.match(/^(wikipedia|wikidata|webarchive)/i)) return false;
      if (lowerLine.match(/^(short description|use \w+ dates)/i)) return false;
      if (lowerLine.match(/^(all articles|articles from)/i)) return false;
      return true;
    })
    .join("\n")
    .trim();
}

/**
 * Clean search engine snippets
 * Removes date prefixes, formatting artifacts, and normalizes text
 */
function cleanSearchSnippet(snippet) {
  if (!snippet) return "";

  let cleaned = snippet
    // Remove date prefixes like "Aug 22, 2020 ·" or "Jan 15, 2024 -"
    .replace(/^[A-Z][a-z]{2,8}\s+\d{1,2},?\s+\d{4}\s*[·\-–—:]\s*/i, "")
    // Remove relative date prefixes like "3 days ago ·"
    .replace(/^\d+\s+(days?|hours?|weeks?|months?|years?)\s+ago\s*[·\-–—:]\s*/i, "")
    // Remove ISO date prefixes
    .replace(/^\d{4}-\d{2}-\d{2}\s*[·\-–—:T]\s*/i, "")
    // Remove leading "..." 
    .replace(/^\.{2,}\s*/, "")
    // Remove trailing "..."
    .replace(/\s*\.{2,}$/, "")
    // Clean up multiple spaces
    .replace(/\s+/g, " ")
    .trim();

  // Apply standard text cleaning
  return cleanText(cleaned);
}

module.exports = {
  cleanText,
  removePageMetadataNoise,
  cleanSearchSnippet,
};
