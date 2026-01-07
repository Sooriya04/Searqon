const axios = require("axios");

const httpClient = axios.create({
  timeout: 15000,
  headers: {
    "User-Agent":
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/121.0 Safari/537.36",
    "Accept-Language": "en-US,en;q=0.9",
    Accept: "text/html"
  },
  validateStatus: status => status >= 200 && status < 300
});

module.exports = httpClient;
