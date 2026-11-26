"""
High-speed HTTP client for Searqon (Sync and Async).
"""

from typing import Any, Dict, List, Optional
import httpx

from .exceptions import (
    SearqonAPIError,
    SearqonBlockedError,
    SearqonConnectionError,
    SearqonError,
    SearqonRateLimitError,
)
from .models import (
    CrawlResult,
    MapResult,
    PipelineResponse,
    ScrapeResult,
    SearchResponse,
)


def _handle_response_error(response: httpx.Response) -> None:
    """Translate HTTP status codes and API payloads into typed exceptions."""
    if response.is_success:
        return

    status_code = response.status_code
    error_msg = f"HTTP {status_code}"
    error_type: Optional[str] = None
    retryable = False
    details: Any = None

    try:
        body = response.json()
        if isinstance(body, dict):
            details = body
            error_msg = body.get("message") or body.get("error") or str(body)
            error_type = body.get("error_code") or body.get("type") or body.get("error")
            retryable = bool(body.get("retryable", False))
    except Exception:
        error_msg = response.text or error_msg

    if status_code == 429:
        raise SearqonRateLimitError(
            message=error_msg,
            status_code=status_code,
            error_type=error_type or "RATE_LIMITED",
            retryable=True,
            details=details,
        )
    if status_code in (401, 403) or (error_type and "BLOCKED" in str(error_type).upper()):
        raise SearqonBlockedError(
            message=error_msg,
            status_code=status_code,
            error_type=error_type,
            retryable=retryable,
            details=details,
        )

    raise SearqonAPIError(
        message=error_msg,
        status_code=status_code,
        error_type=error_type,
        retryable=retryable,
        details=details,
    )


class Searqon:
    """
    Synchronous client for the Searqon Web Intelligence Engine.

    Example:
        >>> from searqon import Searqon
        >>> client = Searqon("http://localhost:7493")
        >>> resp = client.search("latest ai agents", limit=5)
        >>> print(resp.results[0].title)
    """

    def __init__(
        self,
        base_url: str = "http://localhost:7493",
        api_key: Optional[str] = None,
        timeout: float = 30.0,
        client: Optional[httpx.Client] = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        headers = {
            "Content-Type": "application/json",
            "User-Agent": "Searqon-Python-SDK/0.1.0",
        }
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"

        self._client = client or httpx.Client(
            base_url=self.base_url,
            headers=headers,
            timeout=timeout,
        )
        self._owns_client = client is None

    def __enter__(self) -> "Searqon":
        return self

    def __exit__(self, exc_type: Any, exc_val: Any, exc_tb: Any) -> None:
        self.close()

    def close(self) -> None:
        """Close the underlying HTTP client session."""
        if self._owns_client:
            self._client.close()

    def _post(self, endpoint: str, json_data: Dict[str, Any]) -> Any:
        try:
            resp = self._client.post(endpoint, json=json_data)
        except (httpx.ConnectError, httpx.ConnectTimeout, httpx.NetworkError) as err:
            raise SearqonConnectionError(f"Failed to connect to Searqon at {self.base_url}: {err}") from err
        except httpx.TimeoutException as err:
            raise SearqonConnectionError(f"Request to {endpoint} timed out: {err}") from err

        _handle_response_error(resp)
        return resp.json()

    def search(
        self,
        query: str,
        limit: int = 5,
        scrape: bool = True,
        bypass_cache: bool = False,
        max_words: int = 0,
        max_tokens: int = 0,
        summarize: bool = False,
        extract_schema: Optional[str] = None,
    ) -> SearchResponse:
        """
        Execute multi-source search query with optional inline scraping.

        Args:
            query: The search query string.
            limit: Maximum search results to return (clamped 1-10 on server).
            scrape: Whether to fetch and extract full page content.
            bypass_cache: If True, bypass cache and fetch fresh results.
            max_words: Truncate scraped content to this word count (0 for full).
            max_tokens: Truncate content to token budget.
            summarize: If True, generate an AI summary across ranked results.
            extract_schema: Optional structured schema prompt.
        """
        payload: Dict[str, Any] = {
            "query": query,
            "limit": limit,
            "scrape": scrape,
            "bypass_cache": bypass_cache,
            "max_words": max_words,
            "max_tokens": max_tokens,
            "summarize": summarize,
        }
        if extract_schema:
            payload["extract_schema"] = extract_schema

        data = self._post("/search", payload)
        return SearchResponse.model_validate(data)

    def scrape(
        self,
        url: str,
        format: str = "markdown",
        bypass_cache: bool = False,
        max_words: int = 0,
        max_tokens: int = 0,
        chunk: bool = False,
        extract_schema: Optional[str] = None,
        stealth_level: str = "auto",
    ) -> ScrapeResult:
        """
        Scrape a single URL with 4-tier anti-bot escalation.

        Args:
            url: Target web page URL.
            format: Output format ('markdown', 'text', 'html').
            bypass_cache: If True, force re-fetching page.
            max_words: Truncate content word limit.
            max_tokens: Token count limit for content.
            chunk: If True, return pre-chunked semantic passages.
            extract_schema: Optional schema prompt for extraction.
            stealth_level: 'auto', 'fast_http', 'spoofed_headers', 'headless', 'proxy'.
        """
        payload: Dict[str, Any] = {
            "url": url,
            "format": format,
            "bypass_cache": bypass_cache,
            "max_words": max_words,
            "max_tokens": max_tokens,
            "chunk": chunk,
            "stealth_level": stealth_level,
        }
        if extract_schema:
            payload["extract_schema"] = extract_schema

        data = self._post("/scrape", payload)
        return ScrapeResult.model_validate(data)

    def batch_scrape(
        self,
        urls: List[str],
        format: str = "markdown",
        bypass_cache: bool = False,
        max_words: int = 0,
        stealth_level: str = "auto",
    ) -> List[ScrapeResult]:
        """Scrape multiple URLs in parallel."""
        payload = {
            "urls": urls,
            "format": format,
            "bypass_cache": bypass_cache,
            "max_words": max_words,
            "stealth_level": stealth_level,
        }
        data = self._post("/scrape/batch", payload)
        results = data.get("results", data) if isinstance(data, dict) else data
        return [ScrapeResult.model_validate(item) for item in results]

    def pipeline(
        self,
        query: str,
        max_sources: int = 5,
        bypass_cache: bool = False,
    ) -> PipelineResponse:
        """
        Execute one-shot RAG pipeline: Search -> Scrape -> BM25 Chunking.

        Args:
            query: User search query or question.
            max_sources: Max sources to discover and scrape (1-10).
            bypass_cache: If True, bypass cache.
        """
        payload = {
            "query": query,
            "max_sources": max_sources,
            "bypass_cache": bypass_cache,
        }
        data = self._post("/pipeline", payload)
        return PipelineResponse.model_validate(data)

    def map(self, url: str, limit: int = 50) -> MapResult:
        """Discover outbound links and URL hierarchy on a domain."""
        payload = {"url": url, "limit": limit}
        data = self._post("/map", payload)
        return MapResult.model_validate(data)

    def crawl(
        self,
        url: str,
        limit: int = 10,
        depth: int = 1,
        max_depth: int = 2,
    ) -> CrawlResult:
        """Crawl a website up to a specified depth limit."""
        payload = {
            "url": url,
            "limit": limit,
            "depth": depth,
            "max_depth": max_depth,
        }
        data = self._post("/crawl", payload)
        return CrawlResult.model_validate(data)

    def extract(
        self,
        url: str,
        selectors: Optional[Dict[str, str]] = None,
        schema: Optional[str] = None,
        bypass_cache: bool = False,
    ) -> Dict[str, Any]:
        """Extract structured data using CSS selectors or schema."""
        payload: Dict[str, Any] = {
            "url": url,
            "bypass_cache": bypass_cache,
            "selectors": selectors or {},
        }
        if schema:
            payload["schema"] = schema
        return self._post("/extract", payload)

    def health(self) -> Dict[str, Any]:
        """Check Searqon server health status."""
        try:
            resp = self._client.get("/health")
        except Exception as err:
            raise SearqonConnectionError(f"Health check failed: {err}") from err
        _handle_response_error(resp)
        return resp.json()


class AsyncSearqon:
    """
    Asynchronous client for the Searqon Web Intelligence Engine.

    Example:
        >>> import asyncio
        >>> from searqon import AsyncSearqon
        >>> async def main():
        ...     async with AsyncSearqon("http://localhost:7493") as client:
        ...         resp = await client.search("gemini flash", limit=3)
        ...         print(resp.total)
        >>> asyncio.run(main())
    """

    def __init__(
        self,
        base_url: str = "http://localhost:7493",
        api_key: Optional[str] = None,
        timeout: float = 30.0,
        client: Optional[httpx.AsyncClient] = None,
    ) -> None:
        self.base_url = base_url.rstrip("/")
        headers = {
            "Content-Type": "application/json",
            "User-Agent": "Searqon-Python-SDK/0.1.0",
        }
        if api_key:
            headers["Authorization"] = f"Bearer {api_key}"

        self._client = client or httpx.AsyncClient(
            base_url=self.base_url,
            headers=headers,
            timeout=timeout,
        )
        self._owns_client = client is None

    async def __aenter__(self) -> "AsyncSearqon":
        return self

    async def __aexit__(self, exc_type: Any, exc_val: Any, exc_tb: Any) -> None:
        await self.aclose()

    async def aclose(self) -> None:
        """Close the underlying async HTTP client session."""
        if self._owns_client:
            await self._client.aclose()

    async def _post(self, endpoint: str, json_data: Dict[str, Any]) -> Any:
        try:
            resp = await self._client.post(endpoint, json=json_data)
        except (httpx.ConnectError, httpx.ConnectTimeout, httpx.NetworkError) as err:
            raise SearqonConnectionError(f"Failed to connect to Searqon at {self.base_url}: {err}") from err
        except httpx.TimeoutException as err:
            raise SearqonConnectionError(f"Request to {endpoint} timed out: {err}") from err

        _handle_response_error(resp)
        return resp.json()

    async def search(
        self,
        query: str,
        limit: int = 5,
        scrape: bool = True,
        bypass_cache: bool = False,
        max_words: int = 0,
        max_tokens: int = 0,
        summarize: bool = False,
        extract_schema: Optional[str] = None,
    ) -> SearchResponse:
        """Execute async search query with optional inline scraping."""
        payload: Dict[str, Any] = {
            "query": query,
            "limit": limit,
            "scrape": scrape,
            "bypass_cache": bypass_cache,
            "max_words": max_words,
            "max_tokens": max_tokens,
            "summarize": summarize,
        }
        if extract_schema:
            payload["extract_schema"] = extract_schema

        data = await self._post("/search", payload)
        return SearchResponse.model_validate(data)

    async def scrape(
        self,
        url: str,
        format: str = "markdown",
        bypass_cache: bool = False,
        max_words: int = 0,
        max_tokens: int = 0,
        chunk: bool = False,
        extract_schema: Optional[str] = None,
        stealth_level: str = "auto",
    ) -> ScrapeResult:
        """Scrape a single URL asynchronously."""
        payload: Dict[str, Any] = {
            "url": url,
            "format": format,
            "bypass_cache": bypass_cache,
            "max_words": max_words,
            "max_tokens": max_tokens,
            "chunk": chunk,
            "stealth_level": stealth_level,
        }
        if extract_schema:
            payload["extract_schema"] = extract_schema

        data = await self._post("/scrape", payload)
        return ScrapeResult.model_validate(data)

    async def batch_scrape(
        self,
        urls: List[str],
        format: str = "markdown",
        bypass_cache: bool = False,
        max_words: int = 0,
        stealth_level: str = "auto",
    ) -> List[ScrapeResult]:
        """Scrape multiple URLs in parallel asynchronously."""
        payload = {
            "urls": urls,
            "format": format,
            "bypass_cache": bypass_cache,
            "max_words": max_words,
            "stealth_level": stealth_level,
        }
        data = await self._post("/scrape/batch", payload)
        results = data.get("results", data) if isinstance(data, dict) else data
        return [ScrapeResult.model_validate(item) for item in results]

    async def pipeline(
        self,
        query: str,
        max_sources: int = 5,
        bypass_cache: bool = False,
    ) -> PipelineResponse:
        """Execute async one-shot RAG pipeline."""
        payload = {
            "query": query,
            "max_sources": max_sources,
            "bypass_cache": bypass_cache,
        }
        data = await self._post("/pipeline", payload)
        return PipelineResponse.model_validate(data)

    async def map(self, url: str, limit: int = 50) -> MapResult:
        """Discover links and URL hierarchy asynchronously."""
        payload = {"url": url, "limit": limit}
        data = await self._post("/map", payload)
        return MapResult.model_validate(data)

    async def crawl(
        self,
        url: str,
        limit: int = 10,
        depth: int = 1,
        max_depth: int = 2,
    ) -> CrawlResult:
        """Crawl website asynchronously."""
        payload = {
            "url": url,
            "limit": limit,
            "depth": depth,
            "max_depth": max_depth,
        }
        data = await self._post("/crawl", payload)
        return CrawlResult.model_validate(data)

    async def extract(
        self,
        url: str,
        selectors: Optional[Dict[str, str]] = None,
        schema: Optional[str] = None,
        bypass_cache: bool = False,
    ) -> Dict[str, Any]:
        """Extract structured data asynchronously."""
        payload: Dict[str, Any] = {
            "url": url,
            "bypass_cache": bypass_cache,
            "selectors": selectors or {},
        }
        if schema:
            payload["schema"] = schema
        return await self._post("/extract", payload)

    async def health(self) -> Dict[str, Any]:
        """Check server health asynchronously."""
        try:
            resp = await self._client.get("/health")
        except Exception as err:
            raise SearqonConnectionError(f"Health check failed: {err}") from err
        _handle_response_error(resp)
        return resp.json()
