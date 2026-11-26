"""
Typed exceptions for Searqon SDK.
"""

from typing import Any, Optional


class SearqonError(Exception):
    """Base exception for all Searqon errors."""
    pass


class SearqonConnectionError(SearqonError):
    """Raised when unable to connect to the Searqon server instance."""
    pass


class SearqonAPIError(SearqonError):
    """Raised when the Searqon server returns an HTTP error status code."""

    def __init__(
        self,
        message: str,
        status_code: int,
        error_type: Optional[str] = None,
        retryable: bool = False,
        details: Optional[Any] = None,
    ) -> None:
        super().__init__(f"[{status_code}] {message}" if error_type is None else f"[{status_code} {error_type}] {message}")
        self.message = message
        self.status_code = status_code
        self.error_type = error_type
        self.retryable = retryable
        self.details = details


class SearqonRateLimitError(SearqonAPIError):
    """Raised when request is rate limited (HTTP 429)."""
    pass


class SearqonBlockedError(SearqonAPIError):
    """Raised when target scraping is blocked by anti-bot or CAPTCHA."""
    pass
