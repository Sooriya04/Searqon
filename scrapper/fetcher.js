const https = require('https');
const http = require('http');
const { URL } = require('url');

const DEFAULT_USER_AGENT =
  'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36';
const DEFAULT_TIMEOUT_MS = 15000;

function fetchUrl(url, options = {}) {
  return new Promise((resolve, reject) => {
    let parsedUrl;
    try {
      parsedUrl = new URL(url);
    } catch (e) {
      return reject(new Error(`Invalid URL: ${url}`));
    }

    const lib = parsedUrl.protocol === 'https:' ? https : http;
    const requestOptions = {
      method: 'GET',
      headers: {
        'User-Agent': options.userAgent || DEFAULT_USER_AGENT,
        Accept:
          'text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8',
        'Accept-Language': 'en-US,en;q=0.8',
      },
      timeout: options.timeout || DEFAULT_TIMEOUT_MS,
    };

    const req = lib.request(url, requestOptions, (res) => {
      // Handle redirects (basic implementation)
      if (
        res.statusCode >= 300 &&
        res.statusCode < 400 &&
        res.headers.location
      ) {
        // Warning: minimal recursion protection here. You might want to add a loop limit.
        return fetchUrl(res.headers.location, options)
          .then(resolve)
          .catch(reject);
      }

      if (res.statusCode < 200 || res.statusCode >= 300) {
        return reject(new Error(`Status Code: ${res.statusCode}`));
      }

      let data = '';
      res.on('data', (chunk) => {
        data += chunk;
      });

      res.on('end', () => {
        resolve({
          url: url, // or res.responseUrl if available/tracked
          html: data,
          status: res.statusCode,
          contentType: res.headers['content-type'],
        });
      });
    });

    req.on('error', (err) => {
      reject(err);
    });

    req.on('timeout', () => {
      req.destroy();
      reject(new Error('Request Timeout'));
    });

    req.end();
  });
}

module.exports = {
  fetchUrl,
};
