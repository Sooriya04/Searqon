from __future__ import annotations

import asyncio
import logging
from typing import Callable, Coroutine, Any, List, Set
from .models import Request

logger = logging.getLogger(__name__)

class RequestQueue:
    """
    A queue handling deduction (unique_keys), and asynchronous dispatching.
    Mimics Crawlee's RequestQueue storage.
    """
    def __init__(self) -> None:
        self._queue: asyncio.Queue[Request] = asyncio.Queue()
        self._in_progress: Set[str] = set()
        self._handled: Set[str] = set()

    async def add_request(self, request: Request) -> bool:
        """Adds a request if it hasn't been crawled already. Returns True if added."""
        if request.unique_key in self._handled or request.unique_key in self._in_progress:
            return False
            
        self._in_progress.add(request.unique_key)
        await self._queue.put(request)
        return True

    async def add_requests(self, requests: List[Request]) -> None:
        """Convenience method for adding multiple requests."""
        for r in requests:
            await self.add_request(r)

    async def fetch_next_request(self) -> Request | None:
        """Pulls the next request. Returns None if empty."""
        try:
            return self._queue.get_nowait()
        except asyncio.QueueEmpty:
            return None

    async def mark_request_handled(self, request: Request) -> None:
        """Marks a request as fully finished to prevent it ever being crawled again."""
        if request.unique_key in self._in_progress:
            self._in_progress.remove(request.unique_key)
        self._handled.add(request.unique_key)
        
    def reclaim_request(self, request: Request) -> None:
        """If a request fails, it can be put back in the queue for retry."""
        if request.unique_key in self._in_progress:
            self._in_progress.remove(request.unique_key)
        # We don't mark as handled; it needs re-processing
        # Re-adding it bypasses the normal `add_request` check temporarily
        # In a full implementation, we'd handle retry limits before doing this.
        self._queue.put_nowait(request)

    def is_finished(self) -> bool:
        return self._queue.empty() and len(self._in_progress) == 0

class Dataset:
    """
    Simulates storing datasets to JSON lines asynchronously.
    """
    def __init__(self, filename: str = "dataset.jsonl") -> None:
        self.filename = filename
        self._lock = asyncio.Lock()

    async def push_data(self, data: dict[str, Any] | list[dict[str, Any]]) -> None:
        import json
        import os
        
        os.makedirs(os.path.dirname(self.filename) or ".", exist_ok=True)
        
        items = data if isinstance(data, list) else [data]
        
        async with self._lock:
            with open(self.filename, 'a', encoding='utf-8') as f:
                for item in items:
                    f.write(json.dumps(item) + "\n")
