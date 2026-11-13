package utils

import (
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"sync"
	"time"

	"src/db"
)

type cookiePersistTask struct {
	domain      string
	cookiesJSON string
}

var (
	cookiePersistChan  = make(chan cookiePersistTask, 1024)
	cookieWorkerOnce   sync.Once
	cookieFlushTrigger = make(chan chan struct{})
)

func startCookiePersistWorker() {
	cookieWorkerOnce.Do(func() {
		go func() {
			batch := make(map[string]string)
			ticker := time.NewTicker(200 * time.Millisecond)
			defer ticker.Stop()

			flush := func() {
				if len(batch) == 0 {
					return
				}
				for domain, cJSON := range batch {
					_ = db.SaveSessionCookies(domain, cJSON)
				}
				batch = make(map[string]string)
			}

			for {
				select {
				case task := <-cookiePersistChan:
					batch[task.domain] = task.cookiesJSON
					if len(batch) >= 64 {
						flush()
					}
				case <-ticker.C:
					flush()
				case done := <-cookieFlushTrigger:
					flush()
					close(done)
				}
			}
		}()
	})
}

// FlushCookieJarPersistWorker flushes any pending batched cookie writes to the database.
func FlushCookieJarPersistWorker() {
	startCookiePersistWorker()
	done := make(chan struct{})
	select {
	case cookieFlushTrigger <- done:
		<-done
	case <-time.After(2 * time.Second):
	}
}

// PersistentCookieJar implements http.CookieJar and persists cookies asynchronously/batched to the database.
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
	startCookiePersistWorker()
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
// Cookies are immediately updated in memory and queued for batched database persistence
// so concurrent scrapers never block on disk I/O.
func (p *PersistentCookieJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	p.mu.Lock()
	p.jar.SetCookies(u, cookies)
	domain := getDomain(u)
	allCookies := p.jar.Cookies(u)
	p.mu.Unlock()

	if len(allCookies) > 0 {
		if cookiesJSON, err := json.Marshal(allCookies); err == nil {
			startCookiePersistWorker()
			select {
			case cookiePersistChan <- cookiePersistTask{domain: domain, cookiesJSON: string(cookiesJSON)}:
			default:
				// Channel is saturated under heavy concurrent load; persist in background goroutine without blocking caller
				go func(d, c string) {
					_ = db.SaveSessionCookies(d, c)
				}(domain, string(cookiesJSON))
			}
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
