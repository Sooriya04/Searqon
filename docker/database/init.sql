-- Init SQL script to configure Searqon's caching schema on startup

CREATE TABLE IF NOT EXISTS search_cache (
    query TEXT PRIMARY KEY,
    results JSONB NOT NULL,
    provider TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS scrape_cache (
    url TEXT PRIMARY KEY,
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    markdown TEXT NOT NULL,
    word_count INTEGER NOT NULL,
    scraped BOOLEAN DEFAULT TRUE,
    error_msg TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_search_cache_created_at ON search_cache(created_at);
CREATE INDEX IF NOT EXISTS idx_scrape_cache_created_at ON scrape_cache(created_at);
