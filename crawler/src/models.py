from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from enum import Enum
from typing import Any, Callable, Dict, Optional, Coroutine

class RequestState(Enum):
    UNPROCESSED = 0
    BEFORE_NAV = 1
    REQUEST_SENT = 2
    RESPONSE_RECEIVED = 3
    DONE = 4
    ERROR = 5

@dataclass
class Request:
    """
    A robust Request object similar to Crawlee, tracking retries,
    headers, user data, and state.
    """
    url: str
    unique_key: str = field(init=False)
    id: str = field(init=False)
    method: str = 'GET'
    payload: Optional[str | bytes] = None
    headers: Dict[str, str] = field(default_factory=dict)
    user_data: Dict[str, Any] = field(default_factory=dict)
    
    # State tracking
    retry_count: int = 0
    max_retries: int = 3
    handled_at: Optional[datetime] = None
    state: RequestState = RequestState.UNPROCESSED
    
    # The session ID used for this request, if any
    session_id: Optional[str] = None

    def __post_init__(self):
        # In a real app, unique_key is a hash of url + method + payload
        # Here we simplify it slightly for readability while maintaining the interface
        self.unique_key = self.url
        self.id = self.unique_key

@dataclass
class BasicCrawlingContext:
    """Base context passed to handlers."""
    request: Request
    push_data: Callable[[Dict[str, Any]], Coroutine[Any, Any, None]]
    # Enqueue links is typically more complex, simplified for readability
    enqueue_links: Callable[..., Coroutine[Any, Any, None]]
    log: Any # Logger instance

@dataclass
class HttpCrawlingContext(BasicCrawlingContext):
    """Context extended with raw HTTP response."""
    response: Any # aiohttp/httpx response

@dataclass
class BeautifulSoupCrawlingContext(HttpCrawlingContext):
    """Context extended with parsed BeautifulSoup."""
    soup: Any # BeautifulSoup object

