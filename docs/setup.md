# Setup Guide

This guide will help you get Searqon up and running on your local machine.

## Prerequisites

- **Node.js** (v18 or higher)
- **Go** (v1.22 or higher)
- **Python** (v3.9 or higher)
- **Ollama** (v0.6 or higher for Intelligent Routing)

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
The scraping engine runs as a high-performance Go microservice.
```bash
cd go_scraper
# Fetch dependencies
go mod tidy
cd ..
```

### 4. Setup Python Classifier
The intelligence layer uses Ollama for classification.
```bash
# Pull the required model
ollama pull qwen2.5:0.5b

# Install dependencies
pip3 install ollama
```

## Configuration

Create a `settings.yaml` in the root directory (or modify the existing one) to configure timeouts, concurrency, and browser settings.

## Running the Application

Searqon uses `concurrently` to run both the Node.js API and the Go Scraper at the same time.

### Development Mode (with hot-reload)
```bash
npm run dev
```

### Production Mode
```bash
npm start
```

Once started:
- **Main API**: Available at `http://localhost:3001`
- **Go Scraper**: Available at `http://localhost:3002`
- **Intelligent Classifier**: Available at `http://localhost:3003`

## Documentation

- [Scraping Architecture](./scraping.md) - Deep dive into how our Go-based crawler works.
