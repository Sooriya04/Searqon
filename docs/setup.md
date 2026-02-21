# Setup Guide

This guide will help you get Searqon up and running on your local machine.

## Prerequisites

- **Node.js** (v18 or higher)
- **Python** (v3.10 or higher)
- **Pip** (Python package manager)
- **Ollama** (optional, for LLM features)

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

### 3. Setup Python Crawler
The scraping engine runs as a Python microservice.
```bash
cd crawler
# Create a virtual environment (optional but recommended)
python -m venv .venv
source .venv/bin/activate  # On Windows: .venv\Scripts\activate

# Install requirements
pip install -r requirements.txt
cd ..
```

## Configuration

Create a `settings.yaml` in the root directory (or modify the existing one) to configure timeouts, concurrency, and browser settings.

## Running the Application

Searqon uses `concurrently` to run both the Node.js API and the Python Scraper at the same time.

### Development Mode (with hot-reload)
```bash
npm run dev
```

### Production Mode
```bash
npm start
```

Once started, the API will be available at `http://localhost:3001`.

## Documentation

- [Scraping Architecture](./scraping.md) - Deep dive into how our Python-based crawler works.
