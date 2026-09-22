package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"src/config"
	"src/models"
)

var (
	redisClient  *redis.Client
	redisEnabled bool
	redisPrefix  string = "searqon:"
	redisMu      sync.RWMutex
)

type redisSearchEntry struct {
	Results   []models.SearchResult `json:"results"`
	Provider  string                `json:"provider"`
	CreatedAt time.Time             `json:"created_at"`
}

// InitRedis initializes the Redis client if enabled in settings.
func InitRedis(cfg config.RedisConfig) {
	if !cfg.Enabled {
		return
	}

	redisMu.Lock()
	defer redisMu.Unlock()

	host := cfg.Host
	if host == "" {
		host = "localhost"
	}
	port := cfg.Port
	if port <= 0 {
		port = 6379
	}

	if cfg.KeyPrefix != "" {
		redisPrefix = cfg.KeyPrefix
	}

	addr := fmt.Sprintf("%s:%d", host, port)
	client := redis.NewClient(&redis.Options{
		Addr:         addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		log.Printf("[Redis] Ping failed for %s: %v. Redis cache is DISABLED.", addr, err)
		_ = client.Close()
		redisEnabled = false
		return
	}

	redisClient = client
	redisEnabled = true
	log.Printf("[Redis] Connected to Redis successfully at %s (db %d)", addr, cfg.DB)
}

// IsRedisEnabled returns whether Redis cache is operational.
func IsRedisEnabled() bool {
	redisMu.RLock()
	defer redisMu.RUnlock()
	return redisEnabled && redisClient != nil
}

// CloseRedis gracefully terminates the Redis client connection.
func CloseRedis() {
	redisMu.Lock()
	defer redisMu.Unlock()
	if redisEnabled && redisClient != nil {
		_ = redisClient.Close()
		redisEnabled = false
		log.Println("[Redis] Connection closed.")
	}
}

func hashKey(prefix, val string) string {
	sum := sha256.Sum256([]byte(val))
	return prefix + hex.EncodeToString(sum[:16])
}

// RedisGetSearchCache retrieves search results from Redis.
func RedisGetSearchCache(query string) ([]models.SearchResult, string, bool) {
	if !IsRedisEnabled() {
		return nil, "", false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	key := hashKey(redisPrefix+"search:", query)
	data, err := redisClient.Get(ctx, key).Bytes()
	if err != nil {
		return nil, "", false
	}

	var entry redisSearchEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, "", false
	}

	return entry.Results, entry.Provider, true
}

// RedisSaveSearchCache caches search results in Redis with TTL.
func RedisSaveSearchCache(query string, results []models.SearchResult, provider string, ttl time.Duration) {
	if !IsRedisEnabled() {
		return
	}

	entry := redisSearchEntry{
		Results:   results,
		Provider:  provider,
		CreatedAt: time.Now().UTC(),
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	key := hashKey(redisPrefix+"search:", query)
	_ = redisClient.Set(ctx, key, data, ttl).Err()
}

// RedisGetScrapeCache retrieves scraped page result from Redis.
func RedisGetScrapeCache(targetURL string) (models.ScrapeResult, bool) {
	if !IsRedisEnabled() {
		return models.ScrapeResult{}, false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	key := hashKey(redisPrefix+"scrape:", targetURL)
	data, err := redisClient.Get(ctx, key).Bytes()
	if err != nil {
		return models.ScrapeResult{}, false
	}

	var res models.ScrapeResult
	if err := json.Unmarshal(data, &res); err != nil {
		return models.ScrapeResult{}, false
	}

	res.Cached = true
	return res, true
}

// RedisSaveScrapeCache stores scraped page result in Redis with TTL.
func RedisSaveScrapeCache(res models.ScrapeResult, ttl time.Duration) {
	if !IsRedisEnabled() {
		return
	}

	data, err := json.Marshal(res)
	if err != nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	key := hashKey(redisPrefix+"scrape:", res.URL)
	_ = redisClient.Set(ctx, key, data, ttl).Err()
}
