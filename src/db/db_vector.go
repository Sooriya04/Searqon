package db

import (
	"context"
	"log"
	"time"
)

var vectorEnabled bool

// InitVectorDB checks for the pgvector extension and creates the page_embeddings table.
func InitVectorDB() {
	dbMu.RLock()
	enabled := dbEnabled
	pool := dbPool
	dbMu.RUnlock()

	if !enabled || pool == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector;")
	if err != nil {
		log.Printf("[Database] pgvector extension not available/supported: %v. Semantic search is DISABLED.", err)
		vectorEnabled = false
		return
	}

	schema := `
	CREATE TABLE IF NOT EXISTS page_embeddings (
		url TEXT PRIMARY KEY,
		embedding VECTOR(768) NOT NULL,
		created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_page_embeddings_cosine ON page_embeddings USING hnsw (embedding vector_cosine_ops);
	`
	_, err = pool.Exec(ctx, schema)
	if err != nil {
		log.Printf("[Database] Failed to initialize page_embeddings table: %v. Semantic search is DISABLED.", err)
		vectorEnabled = false
		return
	}

	vectorEnabled = true
	log.Println("[Database] pgvector semantic embeddings table initialized successfully.")
}

// SavePageEmbedding stores the vector embedding for a page.
func SavePageEmbedding(targetURL string, embedding []float32) {
	dbMu.RLock()
	enabled := dbEnabled && vectorEnabled
	pool := dbPool
	dbMu.RUnlock()

	if !enabled || pool == nil || len(embedding) == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := pool.Exec(ctx, `
		INSERT INTO page_embeddings (url, embedding, created_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (url) DO UPDATE
		SET embedding = EXCLUDED.embedding, created_at = CURRENT_TIMESTAMP
	`, targetURL, embedding)

	if err != nil {
		log.Printf("[Database] Failed to save embedding for %s: %v", targetURL, err)
	}
}
