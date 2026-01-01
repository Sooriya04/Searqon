const axios = require("axios");

const httpClient = axios.create({
  timeout: 10000,
  headers: {
    "User-Agent":
      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 Chrome/121.0",
    "Accept-Language": "en-US,en;q=0.9"
  },
  validateStatus: () => true
});

module.exports = httpClient;
