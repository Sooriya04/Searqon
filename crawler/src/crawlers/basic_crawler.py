from __future__ import annotations

import logging
from typing import List, Optional

from ..models import Request, BasicCrawlingContext
from ..storage import RequestQueue, Dataset
from ..session_pool import SessionPool
from ..router import Router
from ..autoscaler import Autoscaler

logger = logging.getLogger(__name__)

class BasicCrawler:
    """
    The foundational crawling loop handling queue fetching, 
    session generation, and autoscaled concurrency execution.
    Agnostic to what is actually being fetched (HTTP, local files, etc.).
    """
    def __init__(self,
                 max_concurrency: int = 50,
                 max_request_retries: int = 3):
        self.max_concurrency = max_concurrency
        self.max_request_retries = max_request_retries
        
        # Subsystems
        self.request_queue = RequestQueue()
        self.session_pool = SessionPool()
        self.router = Router()
        
        # We hook the `_run_task_loop` worker directly into the Autoscaler
        self.autoscaler = Autoscaler(self._run_task_loop, max_concurrency=self.max_concurrency)
        
    async def add_requests(self, requests: List[str | Request]) -> None:
        """Helper to seed the crawler."""
        parsed = [Request(url=r) if isinstance(r, str) else r for r in requests]
        await self.request_queue.add_requests(parsed)
        
    async def _handle_request(self, context: BasicCrawlingContext) -> None:
        """
        Executes the router. Must be overridden by subclasses to provide 
        specific context types (like HttpCrawlingContext).
        """
        await self.router.invoke(context)
        
    async def _execute_context_pipeline(self, request: Request, session: Any) -> None:
        """
        Builds the context and invokes the handler. 
        Subclasses (like BeautifulSoupCrawler) override this to inject parsed data.
        """
        context = BasicCrawlingContext(
            request=request,
            push_data=Dataset().push_data,
            enqueue_links=self.request_queue.add_requests,
            log=logger
        )
        await self._handle_request(context)

    async def _run_task_loop(self) -> None:
        """The core worker function fetching requests from the queue and running them."""
        # Get next request
        request = await self.request_queue.fetch_next_request()
        if not request:
            # If queue is momentarily empty, sleep so we don't CPU spike
            # The Autoscaler will eventually kill us if the crawler finishes
            import asyncio
            await asyncio.sleep(0.5)
            return
            
        # Get a Session
        session = self.session_pool.get_session()
        request.session_id = session.id
        
        try:
            logger.info(f"Processing URL: {request.url}")
            await self._execute_context_pipeline(request, session)
            
            # Request successful, mark good
            session.mark_good()
            await self.request_queue.mark_request_handled(request)
            
        except Exception as e:
            logger.warning(f"Failed processing URL: {request.url}, Error: {e}")
            session.mark_bad()
            
            # Retry logic
            request.retry_count += 1
            if request.retry_count < self.max_request_retries:
                logger.info(f"Reclaiming {request.url} for retry {request.retry_count}/{self.max_request_retries}")
                self.request_queue.reclaim_request(request)
            else:
                logger.error(f"Giving up on {request.url} after {self.max_request_retries} retries.")
                await self.request_queue.mark_request_handled(request)
        finally:
            self.request_queue._queue.task_done()

    async def run(self, start_urls: Optional[List[str]] = None) -> None:
        """Main entry point. Seeds URLs and starts the Autoscaler."""
        if start_urls:
            await self.add_requests(start_urls)
            
        logger.info("BasicCrawler initializing Autoscaled execution...")
        await self.autoscaler.start()
        
        # We need a polling mechanism to wait until the Queue is totally drained
        import asyncio
        while not self.request_queue.is_finished():
            await asyncio.sleep(1)
            
        await self.autoscaler.stop()
        logger.info("BasicCrawler run completed successfully.")
