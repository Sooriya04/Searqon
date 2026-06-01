const { BROWSER_HEADERS } = require("./browserHeaders");
const { cleanText, removePageMetadataNoise, cleanSearchSnippet } = require("./textCleaner");

module.exports = {
  BROWSER_HEADERS,
  cleanText,
  removePageMetadataNoise,
  cleanSearchSnippet,
};
