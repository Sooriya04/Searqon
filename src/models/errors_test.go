package models

import (
	"context"
	"errors"
	"testing"
)

func TestClassifyError(t *testing.T) {
	// 1. Timeout error
	timeoutErr := ClassifyError(0, "", context.DeadlineExceeded)
	if timeoutErr == nil || timeoutErr.ErrorType != ErrTimeout || timeoutErr.StatusCode != 504 || !timeoutErr.Retryable || timeoutErr.SuggestedFallback != FallbackRetryWithDelay {
		t.Errorf("Expected TIMEOUT error classification, got %+v", timeoutErr)
	}

	// 2. Rate Limit (429)
	rateErr := ClassifyError(429, "Too many requests", nil)
	if rateErr == nil || rateErr.ErrorType != ErrRateLimited || rateErr.StatusCode != 429 || !rateErr.Retryable || rateErr.SuggestedFallback != FallbackRetryWithDelay {
		t.Errorf("Expected RATE_LIMITED error classification, got %+v", rateErr)
	}

	// 3. Cloudflare Blocked (503 / cloudflare signature)
	cfErr := ClassifyError(503, "Cloudflare Ray ID: 12345 - Just a moment...", nil)
	if cfErr == nil || cfErr.ErrorType != ErrCloudflareBlocked || cfErr.StatusCode != 503 || cfErr.Retryable || cfErr.SuggestedFallback != FallbackLightpanda {
		t.Errorf("Expected CLOUDFLARE_BLOCKED error classification, got %+v", cfErr)
	}

	// 4. Captcha Blocked
	captchaErr := ClassifyError(403, "Please verify you are human to access this site", nil)
	if captchaErr == nil || captchaErr.ErrorType != ErrCaptchaBlocked || captchaErr.StatusCode != 403 || captchaErr.Retryable || captchaErr.SuggestedFallback != FallbackLightpanda {
		t.Errorf("Expected CAPTCHA_BLOCKED error classification, got %+v", captchaErr)
	}

	// 5. Not Found (404)
	notFoundErr := ClassifyError(404, "Page not found", nil)
	if notFoundErr == nil || notFoundErr.ErrorType != ErrNotFound || notFoundErr.StatusCode != 404 || notFoundErr.Retryable || notFoundErr.SuggestedFallback != FallbackNone {
		t.Errorf("Expected NOT_FOUND error classification, got %+v", notFoundErr)
	}

	// 6. Upstream Unavailable (Connection refused)
	connErr := ClassifyError(0, "", errors.New("dial tcp 127.0.0.1:80: connection refused"))
	if connErr == nil || connErr.ErrorType != ErrUpstreamUnavailable || connErr.StatusCode != 502 || connErr.Retryable || connErr.SuggestedFallback != FallbackProxy {
		t.Errorf("Expected UPSTREAM_UNAVAILABLE error classification, got %+v", connErr)
	}
}
