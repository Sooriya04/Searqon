# Searqon Deployment Configuration

Copy and paste the following values directly into your platform's **New Project** fields to deploy the optimized, lightweight version of Searqon.

## 📋 Copy-and-Paste Fields

| Field | Value to Paste |
| :--- | :--- |
| **Project Name** | `Searqon AI Search` |
| **Branch** | `main` |
| **Install Command** | `npm install` |
| **Build Command** | `npm run build:go` |
| **Start Command** | `npx concurrently "node server.js" "./go_scraper_bin"` |
| **Port** | `3001` |
| **Working Directory** | *(Leave blank)* |

---

## 🛠 Prerequisites

> [!IMPORTANT]
> **Host Environment**: The deployment server must have **Node.js** and the **Go compiler** installed. The build command compiles the search engine's scraper into a high-performance binary during deployment.

- **SQLite Support**: The platform should allow persistent storage for the file `database.sqlite` if you want to save search history.
- **LLM Backends**: If using Ollama, ensure it is running on the host or reachable via URL. For Gemini/OpenAI, ensure your API keys are set in the Environment Variables.

---

## 🚀 Post-Deployment Verification

Once deployed, verify the installation by hitting these endpoints:

1.  **Health Check**: `GET /api/v1/health`
2.  **Go Scraper Check**: `GET http://localhost:3002/health` (Internal check)

---

## 💡 Performance Note
By using this configuration, the entire application (Node + Go) will run in approximately **50MB - 80MB RAM**, making it extremely suitable for low-resource VPS or bare-metal setups.
