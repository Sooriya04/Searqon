package models

import "time"

// Session represents a persistent browser session for a specific domain.
type Session struct {
	Domain    string    `json:"domain"`
	Cookies   string    `json:"cookies"` // JSON array of cookies
	UserAgent string    `json:"user_agent"`
	UpdatedAt time.Time `json:"updated_at"`
}
