package db

import (
	"context"
	"encoding/json"
	"log"
	"time"
)

var vectorEnabled bool

// InitVectorDB checks for the pgvector extension (PostgreSQL) or initializes SQLite embeddings table.
func InitVectorDB() {
	dbMu.RLock()
	enabled := dbEnabled
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	if !enabled {
		return
	}

	if backend == "sqlite" && sDB != nil {
		schema := `
		CREATE TABLE IF NOT EXISTS page_embeddings (
			url TEXT PRIMARY KEY,
			embedding TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		`
		_, err := sDB.Exec(schema)
		if err != nil {
			log.Printf("[Database] SQLite page_embeddings table initialization notice: %v", err)
			vectorEnabled = false
			return
		}
		vectorEnabled = true
		log.Println("[Database] SQLite embeddings table initialized successfully.")
		return
	}

	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		_, err := pPool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector;")
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
		_, err = pPool.Exec(ctx, schema)
		if err != nil {
			log.Printf("[Database] Failed to initialize PostgreSQL page_embeddings table: %v. Semantic search is DISABLED.", err)
			vectorEnabled = false
			return
		}

		vectorEnabled = true
		log.Println("[Database] PostgreSQL pgvector semantic embeddings table initialized successfully.")
	}
}

// SavePageEmbedding stores the vector embedding for a page in SQLite or PostgreSQL.
func SavePageEmbedding(targetURL string, embedding []float32) {
	dbMu.RLock()
	enabled := dbEnabled && vectorEnabled
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	if !enabled || len(embedding) == 0 {
		return
	}

	if backend == "sqlite" && sDB != nil {
		embJSON, err := json.Marshal(embedding)
		if err != nil {
			return
		}
		_, err = sDB.Exec(`
			INSERT INTO page_embeddings (url, embedding, created_at)
			VALUES (?, ?, CURRENT_TIMESTAMP)
			ON CONFLICT(url) DO UPDATE SET
				embedding = excluded.embedding,
				created_at = CURRENT_TIMESTAMP
		`, targetURL, string(embJSON))
		if err != nil {
			log.Printf("[Database] SQLite failed to save embedding for %s: %v", targetURL, err)
		}
		return
	}

	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		_, err := pPool.Exec(ctx, `
			INSERT INTO page_embeddings (url, embedding, created_at)
			VALUES ($1, $2, CURRENT_TIMESTAMP)
			ON CONFLICT (url) DO UPDATE
			SET embedding = EXCLUDED.embedding, created_at = CURRENT_TIMESTAMP
		`, targetURL, embedding)

		if err != nil {
			log.Printf("[Database] PostgreSQL failed to save embedding for %s: %v", targetURL, err)
		}
	}
}
