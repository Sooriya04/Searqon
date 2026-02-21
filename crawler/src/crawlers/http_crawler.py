from __future__ import annotations

import logging
from typing import Any

import aiohttp

from ..models import Request, HttpCrawlingContext
from ..storage import Dataset
from .basic_crawler import BasicCrawler

logger = logging.getLogger(__name__)

class AbstractHttpCrawler(BasicCrawler):
    """
    Extends BasicCrawler to perform actual HTTP fetching.
    Handles network errors, retrying blocked sessions (403), and building HttpContext.
    """
    def __init__(self, **kwargs: Any):
        super().__init__(**kwargs)
        # Re-using a single TCP connector is a massive performance boost
        self._session: aiohttp.ClientSession | None = None

    async def _make_http_request(self, request: Request, proxy_session: Any) -> Any:
        """Performs raw I/O."""
        if not self._session:
            # Add reasonable timeouts and browser headers
            timeout = aiohttp.ClientTimeout(total=15)
            self._session = aiohttp.ClientSession(timeout=timeout)
            
        # Standard browser headers to avoid 403 Forbidden
        headers = {
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/121.0.0.0 Safari/537.36",
            "Accept": "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
            "Accept-Language": "en-US,en;q=0.9",
            "Accept-Encoding": "gzip, deflate, br",
            "Cache-Control": "no-cache",
            "Pragma": "no-cache",
            "Sec-Ch-Ua": '"Not A(Brand";v="99", "Google Chrome";v="121", "Chromium";v="121"',
            "Sec-Ch-Ua-Mobile": "?0",
            "Sec-Ch-Ua-Platform": '"Windows"',
            "Sec-Fetch-Dest": "document",
            "Sec-Fetch-Mode": "navigate",
            "Sec-Fetch-Site": "none",
            "Sec-Fetch-User": "?1",
            "Upgrade-Insecure-Requests": "1",
        }
        
        headers.update(request.headers)
        
        logger.debug(f"Fetching {request.url} using session {proxy_session.id}")
        response = await self._session.request(
            method=request.method,
            url=request.url,
            headers=headers,
            data=request.payload,
            allow_redirects=True
        )
        
        # Crawlee anti-block behavior: Handle status codes
        if response.status in {401, 403, 429}:
            proxy_session.retire()
            response.raise_for_status() 
        elif response.status >= 500:
            proxy_session.mark_bad()
            response.raise_for_status() 
            
        return response

    async def _execute_context_pipeline(self, request: Request, session: Any) -> None:
        """Overrides BasicCrawler. Fetches actual HTTP content."""
        response = await self._make_http_request(request, session)
        
        # Build extended context
        context = HttpCrawlingContext(
            request=request,
            push_data=Dataset().push_data,
            enqueue_links=self.request_queue.add_requests,
            log=logger,
            response=response
        )
        
        await self._handle_request(context)

    async def run(self, start_urls: list[str] | None = None) -> None:
        """Ensures HTTP session is closed when we finish."""
        try:
            await super().run(start_urls)
        finally:
            if self._session:
                await self._session.close()
