package db

import (
	"context"
	"log"
	"time"
)

// SaveSessionCookies saves or updates cookies for a specific domain across SQLite or PostgreSQL.
func SaveSessionCookies(domain string, cookiesJSON string) error {
	if !DbEnabled() {
		return nil
	}

	dbMu.RLock()
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	if backend == "sqlite" && sDB != nil {
		_, err := sDB.Exec(`
			INSERT INTO sessions (domain, cookies, user_agent, updated_at)
			VALUES (?, ?, '', CURRENT_TIMESTAMP)
			ON CONFLICT(domain) DO UPDATE SET
				cookies = excluded.cookies,
				updated_at = CURRENT_TIMESTAMP;
		`, domain, cookiesJSON)
		if err != nil {
			log.Printf("[Database] SQLite failed to save session cookies for %s: %v", domain, err)
			return err
		}
		return nil
	}

	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		query := `
			INSERT INTO sessions (domain, cookies, updated_at)
			VALUES ($1, $2, CURRENT_TIMESTAMP)
			ON CONFLICT (domain) DO UPDATE 
			SET cookies = EXCLUDED.cookies, updated_at = CURRENT_TIMESTAMP;
		`
		_, err := pPool.Exec(ctx, query, domain, cookiesJSON)
		if err != nil {
			log.Printf("[Database] PostgreSQL failed to save session cookies for %s: %v", domain, err)
			return err
		}
	}

	return nil
}

// GetSessionCookies retrieves the cookies JSON for a given domain from SQLite or PostgreSQL.
func GetSessionCookies(domain string) (string, error) {
	if !DbEnabled() {
		return "", nil
	}

	dbMu.RLock()
	backend := dbBackend
	sDB := sqliteDB
	pPool := pgPool
	dbMu.RUnlock()

	if backend == "sqlite" && sDB != nil {
		var cookiesJSON string
		err := sDB.QueryRow("SELECT cookies FROM sessions WHERE domain = ?", domain).Scan(&cookiesJSON)
		if err != nil {
			return "", err
		}
		return cookiesJSON, nil
	}

	if backend == "postgres" && pPool != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var cookiesJSON string
		query := `SELECT cookies FROM sessions WHERE domain = $1;`
		err := pPool.QueryRow(ctx, query, domain).Scan(&cookiesJSON)
		if err != nil {
			return "", err
		}
		return cookiesJSON, nil
	}

	return "", nil
}
