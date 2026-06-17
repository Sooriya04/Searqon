package handlers

import (
	"fmt"
	"net/http"
)

const openAPISpec = `{
  "openapi": "3.0.3",
  "info": {
    "title": "Searqon API",
    "description": "Unified Search and Scrape Web Intelligence Engine",
    "version": "2.0.0"
  },
  "paths": {
    "/search": {
      "post": {
        "summary": "Search the web and scrape top result pages",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/SearchRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Search and scrape results",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/APIResponse"
                }
              }
            }
          }
        }
      }
    },
    "/scrape": {
      "post": {
        "summary": "Scrape a single URL",
        "requestBody": {
          "required": true,
          "content": {
            "application/json": {
              "schema": {
                "$ref": "#/components/schemas/ScrapeRequest"
              }
            }
          }
        },
        "responses": {
          "200": {
            "description": "Scraped page content",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/APIResponse"
                }
              }
            }
          }
        }
      }
    },
    "/stats": {
      "get": {
        "summary": "Get system performance and cache metrics",
        "responses": {
          "200": {
            "description": "Performance statistics",
            "content": {
              "application/json": {
                "schema": {
                  "$ref": "#/components/schemas/APIResponse"
                }
              }
            }
          }
        }
      }
    }
  },
  "components": {
    "schemas": {
      "APIResponse": {
        "type": "object",
        "properties": {
          "success": { "type": "boolean" },
          "data": { "type": "object" },
          "error": {
            "type": "object",
            "properties": {
              "code": { "type": "string" },
              "message": { "type": "string" }
            }
          }
        }
      },
      "SearchRequest": {
        "type": "object",
        "required": ["query"],
        "properties": {
          "query": { "type": "string" },
          "limit": { "type": "integer", "default": 5 },
          "scrape": { "type": "boolean", "default": true },
          "bypass_cache": { "type": "boolean", "default": false },
          "max_words": { "type": "integer" },
          "summarize": { "type": "boolean", "default": false }
        }
      },
      "ScrapeRequest": {
        "type": "object",
        "required": ["url"],
        "properties": {
          "url": { "type": "string" },
          "format": { "type": "string", "default": "markdown" },
          "bypass_cache": { "type": "boolean", "default": false },
          "max_words": { "type": "integer" }
        }
      }
    }
  }
}`

const swaggerUIHTML = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>Searqon API Documentation</title>
  <link rel="stylesheet" href="https://unpkg.com/swagger-ui-dist@5/swagger-ui.css" />
  <style>
    body { margin: 0; background-color: #111216; }
    .swagger-ui { filter: invert(88%) hue-rotate(180deg); }
    .swagger-ui .topbar { display: none; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src="https://unpkg.com/swagger-ui-dist@5/swagger-ui-bundle.js"></script>
  <script>
    window.onload = () => {
      window.ui = SwaggerUIBundle({
        url: '/openapi.json',
        dom_id: '#swagger-ui',
        deepLinking: true,
        presets: [SwaggerUIBundle.presets.apis],
      });
    };
  </script>
</body>
</html>`

// OpenAPIHandler serves the raw JSON OpenAPI specification file.
func OpenAPIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	fmt.Fprint(w, openAPISpec)
}

// SwaggerUIHandler serves the OpenAPI interactive UI page.
func SwaggerUIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	fmt.Fprint(w, swaggerUIHTML)
}
