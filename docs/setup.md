# Setup Guide

This guide will help you get Searqon up and running on your local machine.

## Prerequisites

- **Node.js** (v18 or higher)
- **Go** (v1.22 or higher)
- **Ollama** (v0.6 or higher - optional, for AI Smart Answers)

## Installation

### 1. Clone the repository
```bash
git clone https://github.com/Sooriya04/Searqon.git
cd Searqon
```

### 2. Install Node.js dependencies
```bash
npm install
```

### 3. Setup Go Scraper
The scraping engine runs as a high-performance Go binary.
```bash
cd go_scraper
# Fetch dependencies
go mod tidy
cd ..
```

---

## Configuration

Create a `settings.yaml` in the root directory (or modify the existing one) to configure timeouts, concurrency, and search engine parameters.

### LLM Integration (Optional)
If you want to use the **Smart Answer** or **Chat** features, ensure you have the required environment variables in your `.env` file:
- `EXTRACTION_BACKEND` (ollama, gemini, or openai)
- API Keys as required.

---

## Running the Application

Searqon is optimized for a **2-process architecture**. It uses `concurrently` to run both the Node.js API and the Go Scraper.

### Development Mode (with hot-reload)
```bash
npm run dev
```

### Production Mode
The production mode automatically builds the Go scraper into a native binary for maximum performance.
```bash
npm start
```

Once started:
- **Main API**: Available at `http://localhost:3001`
- **Go Scraper**: Available at `http://localhost:3002` (Internal Service)

---

## Project Specifications

- **Target Idle RAM**: < 50MB
- **Classification Latency**: < 1ms (Native JS Engine)
- **Architecture**: Orchestrated Node.js + Compiled Go Binary

---

## Documentation

- [Unified Search API](./search_api.md) - How to use the main search intelligence endpoint.
- [Scraping Architecture](./scraping.md) - Deep dive into how our Go-based extraction works.
