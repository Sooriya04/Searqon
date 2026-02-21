from __future__ import annotations

import logging
from typing import Any
from bs4 import BeautifulSoup

from ..models import Request, BeautifulSoupCrawlingContext
from .http_crawler import AbstractHttpCrawler
from ..storage import Dataset

logger = logging.getLogger(__name__)

class BeautifulSoupCrawler(AbstractHttpCrawler):
    """
    Final concrete class providing a BeautifulSoup parser 
    step to the HttpCrawlingContext.
    """
    
    async def _execute_context_pipeline(self, request: Request, session: Any) -> None:
        """Fetch via HTTP crawler, then parse with BeautifulSoup."""
        response = await self._make_http_request(request, session)
        
        content_type = response.headers.get('Content-Type', '').lower()
        if 'text/html' not in content_type and 'application/xhtml+xml' not in content_type:
            logger.warning(f"Skipping non-HTML content type: {content_type} for {request.url}")
            # Just push empty data to avoid hanging
            context = BeautifulSoupCrawlingContext(
                request=request,
                push_data=Dataset().push_data,
                enqueue_links=self.request_queue.add_requests,
                log=logger,
                response=response,
                soup=BeautifulSoup("", "html.parser")
            )
            await self._handle_request(context)
            return

        # Read raw text
        try:
            html = await response.text()
        except Exception as e:
            logger.error(f"Failed to decode text for {request.url}: {e}")
            raise

        # Parse it
        soup = BeautifulSoup(html, "html.parser")
        
        # Build enriched context
        context = BeautifulSoupCrawlingContext(
            request=request,
            push_data=Dataset().push_data,
            enqueue_links=self.request_queue.add_requests,
            log=logger,
            response=response,
            soup=soup
        )
        
        await self._handle_request(context)

