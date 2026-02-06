const { Readability } = require('@mozilla/readability');
const { parseHTML } = require('linkedom');

function decodeEntities(value) {
  return value
    .replace(/&nbsp;/gi, ' ')
    .replace(/&amp;/gi, '&')
    .replace(/&quot;/gi, '"')
    .replace(/&#39;/gi, "'")
    .replace(/&lt;/gi, '<')
    .replace(/&gt;/gi, '>')
    .replace(/&#x([0-9a-f]+);/gi, (_, hex) =>
      String.fromCharCode(Number.parseInt(hex, 16)),
    )
    .replace(/&#(\d+);/gi, (_, dec) =>
      String.fromCharCode(Number.parseInt(dec, 10)),
    );
}

function stripTags(value) {
  return decodeEntities(value.replace(/<[^>]+>/g, ''));
}

function normalizeWhitespace(value) {
  return value
    .replace(/\r/g, '')
    .replace(/[ \t]+\n/g, '\n')
    .replace(/\n{3,}/g, '\n\n')
    .replace(/[ \t]{2,}/g, ' ')
    .trim();
}

function htmlToMarkdown(html) {
  const titleMatch = html.match(/<title[^>]*>([\s\S]*?)<\/title>/i);
  const title = titleMatch
    ? normalizeWhitespace(stripTags(titleMatch[1]))
    : undefined;

  let text = html
    .replace(/<script[\s\S]*?<\/script>/gi, '')
    .replace(/<style[\s\S]*?<\/style>/gi, '')
    .replace(/<noscript[\s\S]*?<\/noscript>/gi, '');

  text = text.replace(
    /<a\s+[^>]*href=["']([^"']+)["'][^>]*>([\s\S]*?)<\/a>/gi,
    (_, href, body) => {
      const label = normalizeWhitespace(stripTags(body));
      return label ? `[${label}](${href})` : href;
    },
  );

  text = text.replace(
    /<h([1-6])[^>]*>([\s\S]*?)<\/h\1>/gi,
    (_, level, body) => {
      const prefix = '#'.repeat(
        Math.max(1, Math.min(6, Number.parseInt(level, 10))),
      );
      const label = normalizeWhitespace(stripTags(body));
      return `\n${prefix} ${label}\n`;
    },
  );

  text = text.replace(/<li[^>]*>([\s\S]*?)<\/li>/gi, (_, body) => {
    const label = normalizeWhitespace(stripTags(body));
    return label ? `\n- ${label}` : '';
  });

  text = text
    .replace(/<(br|hr)\s*\/?>/gi, '\n')
    .replace(
      /<\/(p|div|section|article|header|footer|table|tr|ul|ol)>/gi,
      '\n',
    );

  text = stripTags(text);
  return { text: normalizeWhitespace(text), title };
}

function markdownToText(markdown) {
  let text = markdown;
  text = text.replace(/!\[[^\]]*]\([^)]+\)/g, ''); // Remove images
  text = text.replace(/\[([^\]]+)]\([^)]+\)/g, '$1'); // Keep link text
  text = text.replace(/```[\s\S]*?```/g, ''); // Remove code blocks
  text = text.replace(/`([^`]+)`/g, '$1'); // Inline code
  text = text.replace(/^#{1,6}\s+/gm, ''); // Headers
  text = text.replace(/^\s*[-*+]\s+/gm, ''); // Lists
  return normalizeWhitespace(text);
}

async function extractReadableContent(
  html,
  url,
  mode = 'markdown',
  options = {},
) {
  // Helper to convert HTML string to Markdown using regex (legacy/fallback)
  const runHtmlToMarkdown = (htmlContent, title) => {
    const rendered = htmlToMarkdown(htmlContent);
    // Use provided title if regex didn't find one or to override
    if (title) rendered.title = title;

    if (mode === 'text') {
      const text =
        markdownToText(rendered.text) ||
        normalizeWhitespace(stripTags(htmlContent));
      return { text, title: rendered.title };
    }
    return rendered;
  };

  try {
    const { document } = parseHTML(html);

    // Base URI mock
    if (url) {
      try {
        document.baseURI = url;
      } catch (e) {}
    }

    // Cleaning logic common to both fullPage and Readability pre-processing
    const cleaner = () => {
      // Remove widely used clutter selectors
      const selectorsToRemove = [
        'script',
        'style',
        'noscript',
        'iframe',
        'svg',
        'nav',
        'footer',
        'header',
        'aside',
        '.nav',
        '.navbar',
        '.navigation',
        '.menu',
        '.sidebar',
        '.footer',
        '.site-footer',
        '.colophon',
        '.ad',
        '.ads',
        '.advertisement',
        '.banner',
        '.cookie-consent',
        '.popup',
        '.search-form',
        '.search-bar',
        '#search',
        '.login',
        '.signup',
        '.auth',
        '.social-share',
        '.share-buttons',
        '[role="navigation"]',
        '[role="banner"]',
        '[role="contentinfo"]',
        '[role="search"]',
        // PubMed specific
        '.usa-banner',
        '.ncbi-header',
        '.ncbi-footer',
        '.bk-portlet',
        '.stand-alone-search-form',
        '.manage-links',
        '.back-to-top',
      ];

      selectorsToRemove.forEach((sel) => {
        try {
          document.querySelectorAll(sel).forEach((el) => el.remove());
        } catch (e) {
          // Ignore invalid selector errors
        }
      });
    };

    // If fullPage option is requested, we stick to the DOM but clean it first
    if (options.fullPage) {
      cleaner();
      // Get the cleaned HTML from body
      const cleanedHtml = document.body.innerHTML;
      const title = document.title;
      return runHtmlToMarkdown(cleanedHtml, title);
    }

    // Check if Readability can be instantiated
    if (!Readability) return runHtmlToMarkdown(html);

    const reader = new Readability(document, { charThreshold: 0 });
    const parsed = reader.parse();

    if (!parsed || !parsed.content) return runHtmlToMarkdown(html);

    const title = parsed.title || undefined;

    if (mode === 'text') {
      const text = normalizeWhitespace(parsed.textContent || '');
      return text ? { text, title } : runHtmlToMarkdown(html);
    }

    // Convert Readability's clean HTML to Markdown
    const rendered = htmlToMarkdown(parsed.content);
    return { text: rendered.text, title: title || rendered.title };
  } catch (error) {
    console.error('Readability/Parsing failed, using raw fallback:', error);
    return runHtmlToMarkdown(html);
  }
}

module.exports = {
  extractReadableContent,
  htmlToMarkdown,
  markdownToText,
};
