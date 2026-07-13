package utils

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"

	"src/db"
)

// PersistentCookieJar implements http.CookieJar and persists cookies to the PostgreSQL database via db package
type PersistentCookieJar struct {
	jar *cookiejar.Jar
	mu  sync.Mutex
}

// NewPersistentCookieJar creates a new persistent cookie jar
func NewPersistentCookieJar() (*PersistentCookieJar, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	return &PersistentCookieJar{
		jar: jar,
	}, nil
}

func getDomain(u *url.URL) string {
	domain := u.Hostname()
	if strings.HasPrefix(domain, "www.") {
		domain = domain[4:]
	}
	return domain
}

// SetCookies handles the receipt of the cookies in a reply for the given URL.
func (p *PersistentCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Update the in-memory jar
	p.jar.SetCookies(u, cookies)

	domain := getDomain(u)
	
	// Get all aggregated cookies for the domain to save
	allCookies := p.jar.Cookies(u)
	
	if len(allCookies) > 0 {
		cookiesJSON, err := json.Marshal(allCookies)
		if err == nil {
			// Save asynchronously or synchronously. Here we do it synchronously to ensure it's written
			_ = db.SaveSessionCookies(domain, string(cookiesJSON))
		}
	}
}

// Cookies returns the cookies to send in a request for the given URL.
// It will try to load from the DB if memory is empty.
func (p *PersistentCookieJar) Cookies(u *url.URL) []*http.Cookie {
	p.mu.Lock()
	defer p.mu.Unlock()

	domain := getDomain(u)
	
	// Check memory jar first
	memCookies := p.jar.Cookies(u)
	if len(memCookies) > 0 {
		return memCookies
	}

	// Fallback to database if nothing in memory
	cookiesJSON, err := db.GetSessionCookies(domain)
	if err == nil && cookiesJSON != "" && cookiesJSON != "[]" {
		var dbCookies []*http.Cookie
		if err := json.Unmarshal([]byte(cookiesJSON), &dbCookies); err == nil {
			// Pre-populate memory jar with what we got from DB
			p.jar.SetCookies(u, dbCookies)
			return dbCookies
		}
	}

	return nil
}
