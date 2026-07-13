package db

import (
	"context"
	"log"
	"time"
)

// SaveSessionCookies saves or updates cookies for a specific domain.
func SaveSessionCookies(domain string, cookiesJSON string) error {
	if !DbEnabled() {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	query := `
		INSERT INTO sessions (domain, cookies, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (domain) DO UPDATE 
		SET cookies = EXCLUDED.cookies, updated_at = CURRENT_TIMESTAMP;
	`
	_, err := dbPool.Exec(ctx, query, domain, cookiesJSON)
	if err != nil {
		log.Printf("[Database] Failed to save session cookies for %s: %v", domain, err)
		return err
	}
	return nil
}

// GetSessionCookies retrieves the cookies JSON for a given domain.
func GetSessionCookies(domain string) (string, error) {
	if !DbEnabled() {
		return "", nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var cookiesJSON string
	query := `SELECT cookies FROM sessions WHERE domain = $1;`
	
	err := dbPool.QueryRow(ctx, query, domain).Scan(&cookiesJSON)
	if err != nil {
		return "", err
	}
	
	return cookiesJSON, nil
}
