package models

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
)

type ErrorCode string

const (
	ErrCaptchaBlocked      ErrorCode = "CAPTCHA_BLOCKED"
	ErrCloudflareBlocked   ErrorCode = "CLOUDFLARE_BLOCKED"
	ErrRateLimited         ErrorCode = "RATE_LIMITED"
	ErrTimeout             ErrorCode = "TIMEOUT"
	ErrUpstreamUnavailable ErrorCode = "UPSTREAM_UNAVAILABLE"
	ErrNotFound            ErrorCode = "NOT_FOUND"
	ErrParsingFailed       ErrorCode = "PARSING_FAILED"
)

type FallbackType string

const (
	FallbackLightpanda     FallbackType = "lightpanda"
	FallbackProxy          FallbackType = "proxy"
	FallbackNone           FallbackType = "none"
	FallbackRetryWithDelay FallbackType = "retry_with_delay"
)

type SearqonError struct {
	ErrorType         ErrorCode              `json:"error"`
	StatusCode        int                    `json:"status_code"`
	Message           string                 `json:"message"`
	Retryable         bool                   `json:"retryable"`
	SuggestedFallback FallbackType           `json:"suggested_fallback"`
	Details           map[string]interface{} `json:"details,omitempty"`
}

func (e *SearqonError) Error() string {
	return fmt.Sprintf("[%s] HTTP %d: %s (retryable: %v, fallback: %s)", e.ErrorType, e.StatusCode, e.Message, e.Retryable, e.SuggestedFallback)
}

// ClassifyError analyzes an error, HTTP status code, and response body/headers to produce a deterministic SearqonError.
func ClassifyError(statusCode int, body string, err error) *SearqonError {
	lowerBody := strings.ToLower(body)

	// 1. Check Go execution errors (timeout, DNS, connection refused)
	if err != nil {
		errStr := strings.ToLower(err.Error())
		if errors.Is(err, context.DeadlineExceeded) || strings.Contains(errStr, "deadline exceeded") || strings.Contains(errStr, "timeout") {
			return &SearqonError{
				ErrorType:         ErrTimeout,
				StatusCode:        http.StatusGatewayTimeout,
				Message:           fmt.Sprintf("Request timed out: %v", err),
				Retryable:         true,
				SuggestedFallback: FallbackRetryWithDelay,
				Details:           map[string]interface{}{"raw_error": err.Error()},
			}
		}

		var netErr net.Error
		if errors.As(err, &netErr) && netErr.Timeout() {
			return &SearqonError{
				ErrorType:         ErrTimeout,
				StatusCode:        http.StatusGatewayTimeout,
				Message:           fmt.Sprintf("Network operation timed out: %v", err),
				Retryable:         true,
				SuggestedFallback: FallbackRetryWithDelay,
				Details:           map[string]interface{}{"raw_error": err.Error()},
			}
		}

		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) || strings.Contains(errStr, "no such host") || strings.Contains(errStr, "dns") {
			return &SearqonError{
				ErrorType:         ErrUpstreamUnavailable,
				StatusCode:        http.StatusBadGateway,
				Message:           fmt.Sprintf("DNS lookup or upstream network failure: %v", err),
				Retryable:         false,
				SuggestedFallback: FallbackNone,
				Details:           map[string]interface{}{"raw_error": err.Error()},
			}
		}

		if strings.Contains(errStr, "connection refused") || strings.Contains(errStr, "connection reset") {
			return &SearqonError{
				ErrorType:         ErrUpstreamUnavailable,
				StatusCode:        http.StatusBadGateway,
				Message:           fmt.Sprintf("Upstream connection refused or reset: %v", err),
				Retryable:         false,
				SuggestedFallback: FallbackProxy,
				Details:           map[string]interface{}{"raw_error": err.Error()},
			}
		}
	}

	// 2. Check HTTP status codes
	if statusCode == 429 || strings.Contains(lowerBody, "too many requests") || strings.Contains(lowerBody, "rate limit") {
		return &SearqonError{
			ErrorType:         ErrRateLimited,
			StatusCode:        429,
			Message:           "Rate limit exceeded (HTTP 429)",
			Retryable:         true,
			SuggestedFallback: FallbackRetryWithDelay,
			Details:           map[string]interface{}{"status_code": statusCode},
		}
	}

	if statusCode == 404 || strings.Contains(lowerBody, "page not found") || strings.Contains(lowerBody, "404 not found") {
		return &SearqonError{
			ErrorType:         ErrNotFound,
			StatusCode:        404,
			Message:           "Target resource not found (HTTP 404)",
			Retryable:         false,
			SuggestedFallback: FallbackNone,
			Details:           map[string]interface{}{"status_code": statusCode},
		}
	}

	// 3. Cloudflare detection
	if strings.Contains(lowerBody, "cloudflare") || strings.Contains(lowerBody, "just a moment") ||
		strings.Contains(lowerBody, "cf-browser-verification") || strings.Contains(lowerBody, "cf_chl_") ||
		strings.Contains(lowerBody, "cf-turnstile") || strings.Contains(lowerBody, "attention required! | cloudflare") {
		sc := statusCode
		if sc <= 0 {
			sc = 403
		}
		return &SearqonError{
			ErrorType:         ErrCloudflareBlocked,
			StatusCode:        sc,
			Message:           "Blocked by Cloudflare challenge / Turnstile",
			Retryable:         false,
			SuggestedFallback: FallbackLightpanda,
			Details:           map[string]interface{}{"status_code": statusCode, "provider": "cloudflare"},
		}
	}

	// 4. Captcha / Bot detection (Datadome, PerimeterX, reCAPTCHA, hCaptcha, Arkose, etc.)
	if strings.Contains(lowerBody, "verify you are human") || strings.Contains(lowerBody, "verify you're a human") ||
		strings.Contains(lowerBody, "datadome") || strings.Contains(lowerBody, "perimeterx") || strings.Contains(lowerBody, "px-captcha") ||
		strings.Contains(lowerBody, "arkoselabs") || strings.Contains(lowerBody, "g-recaptcha") || strings.Contains(lowerBody, "hcaptcha") ||
		strings.Contains(lowerBody, "unusual traffic from your computer network") {
		sc := statusCode
		if sc <= 0 {
			sc = 403
		}
		return &SearqonError{
			ErrorType:         ErrCaptchaBlocked,
			StatusCode:        sc,
			Message:           "Blocked by CAPTCHA or bot challenge",
			Retryable:         false,
			SuggestedFallback: FallbackLightpanda,
			Details:           map[string]interface{}{"status_code": statusCode},
		}
	}

	if statusCode == 403 || statusCode == 503 {
		return &SearqonError{
			ErrorType:         ErrCloudflareBlocked,
			StatusCode:        statusCode,
			Message:           fmt.Sprintf("Access forbidden or service unavailable (HTTP %d)", statusCode),
			Retryable:         false,
			SuggestedFallback: FallbackLightpanda,
			Details:           map[string]interface{}{"status_code": statusCode},
		}
	}

	if statusCode >= 500 {
		return &SearqonError{
			ErrorType:         ErrUpstreamUnavailable,
			StatusCode:        statusCode,
			Message:           fmt.Sprintf("Upstream server error (HTTP %d)", statusCode),
			Retryable:         true,
			SuggestedFallback: FallbackRetryWithDelay,
			Details:           map[string]interface{}{"status_code": statusCode},
		}
	}

	if err != nil {
		return &SearqonError{
			ErrorType:         ErrParsingFailed,
			StatusCode:        500,
			Message:           fmt.Sprintf("Operation failed: %v", err),
			Retryable:         false,
			SuggestedFallback: FallbackNone,
			Details:           map[string]interface{}{"raw_error": err.Error()},
		}
	}

	return nil
}
