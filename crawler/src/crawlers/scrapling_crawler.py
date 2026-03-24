from __future__ import annotations

import logging
from typing import Any
from scrapling import Selector

from ..models import Request, ScraplingCrawlingContext
from .http_crawler import AbstractHttpCrawler
from ..storage import Dataset

logger = logging.getLogger(__name__)

class ScraplingCrawler(AbstractHttpCrawler):
    """
    Optimized crawler: aiohttp for fast fetching + Scrapling Selector for parsing.
    Keeps memory low and latency under 1-2s for most pages.
    """
    
    async def _execute_context_pipeline(self, request: Request, session: Any) -> None:
        """Fetch via aiohttp (fast), then parse with Scrapling Selector."""
        response = await self._make_http_request(request, session)
        
        content_type = response.headers.get('Content-Type', '').lower()
        if 'text/html' not in content_type and 'application/xhtml+xml' not in content_type:
            logger.warning(f"Skipping non-HTML content type: {content_type} for {request.url}")
            context = ScraplingCrawlingContext(
                request=request,
                push_data=Dataset().push_data,
                enqueue_links=self.request_queue.add_requests,
                log=logger,
                response=response,
                page=Selector(content="<html></html>", url=request.url)
            )
            await self._handle_request(context)
            return

        # Read raw text
        try:
            html = await response.text()
        except Exception as e:
            logger.error(f"Failed to decode text for {request.url}: {e}")
            raise

        # Parse with Scrapling's high-performance Selector (lxml-backed)
        page = Selector(content=html, url=request.url)
        
        # Build enriched context
        context = ScraplingCrawlingContext(
            request=request,
            push_data=Dataset().push_data,
            enqueue_links=self.request_queue.add_requests,
            log=logger,
            response=response,
            page=page
        )
        
        await self._handle_request(context)
