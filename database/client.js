const axios = require("axios");

const DB_SERVICE_URL = process.env.DB_SERVICE_URL || "http://localhost:43221";

/**
 * Database Client for communicating with the database microservice
 */
class DatabaseClient {
  /**
   * Save a single result to the database
   * @param {Object} resultData - Result data to save
   * @returns {Promise<Object>} Response from database service
   */
  static async saveResult(resultData) {
    try {
      const response = await axios.post(`${DB_SERVICE_URL}/results`, resultData, {
        headers: { "Content-Type": "application/json" },
        timeout: 5000
      });
      return response.data;
    } catch (err) {
      console.error("[DB Client] Save failed:", err.message);
      // Don't throw - allow the service to continue even if DB save fails
      return { success: false, error: err.message };
    }
  }

  /**
   * Save multiple results to the database (bulk insert)
   * @param {Array} results - Array of result data
   * @returns {Promise<Object>} Response from database service
   */
  static async saveBulkResults(results) {
    try {
      const response = await axios.post(`${DB_SERVICE_URL}/results/bulk`, 
        { results },
        {
          headers: { "Content-Type": "application/json" },
          timeout: 10000
        }
      );
      return response.data;
    } catch (err) {
      console.error("[DB Client] Bulk save failed:", err.message);
      return { success: false, error: err.message };
    }
  }

  /**
   * Search for results in the database
   * @param {string} query - Search query
   * @param {string} source - Source filter (optional)
   * @param {number} limit - Max results to return
   * @returns {Promise<Object>} Search results
   */
  static async searchResults(query, source = null, limit = 10) {
    try {
      const params = { query, limit };
      if (source) params.source = source;

      const response = await axios.get(`${DB_SERVICE_URL}/results/search`, {
        params,
        timeout: 5000
      });
      return response.data;
    } catch (err) {
      console.error("[DB Client] Search failed:", err.message);
      return { success: false, error: err.message, results: [] };
    }
  }

  /**
   * Get all results with pagination
   * @param {number} page - Page number
   * @param {number} limit - Results per page
   * @returns {Promise<Object>} Paginated results
   */
  static async getResults(page = 1, limit = 10) {
    try {
      const response = await axios.get(`${DB_SERVICE_URL}/results`, {
        params: { page, limit },
        timeout: 5000
      });
      return response.data;
    } catch (err) {
      console.error("[DB Client] Fetch failed:", err.message);
      return { success: false, error: err.message, results: [] };
    }
  }

  /**
   * Check database service health
   * @returns {Promise<Object>} Health status
   */
  static async healthCheck() {
    try {
      const response = await axios.get(`${DB_SERVICE_URL}/health`, {
        timeout: 3000
      });
      return response.data;
    } catch (err) {
      console.error("[DB Client] Health check failed:", err.message);
      return { status: "error", error: err.message };
    }
  }

  /**
   * Get database statistics
   * @returns {Promise<Object>} Database stats
   */
  static async getStats() {
    try {
      const response = await axios.get(`${DB_SERVICE_URL}/stats`, {
        timeout: 5000
      });
      return response.data;
    } catch (err) {
      console.error("[DB Client] Stats failed:", err.message);
      return { success: false, error: err.message };
    }
  }
}

module.exports = DatabaseClient;
