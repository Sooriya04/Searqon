from __future__ import annotations

import random
import uuid
import logging
from typing import Dict, List, Optional
from datetime import datetime, timezone, timedelta

logger = logging.getLogger(__name__)

class Session:
    """Represents a single crawling session maintaining its own state and cookies."""
    
    def __init__(self,
                 max_usage_count: int = 50,
                 max_error_score: float = 3.0,
                 error_score_decrement: float = 0.5):
        self.id: str = str(uuid.uuid4())
        self.usage_count: int = 0
        self.max_usage_count: int = max_usage_count
        
        self.error_score: float = 0.0
        self.max_error_score: float = max_error_score
        self.error_score_decrement: float = error_score_decrement
        
        # Simplified cookie jar for readability
        self.cookies: Dict[str, str] = {}
        self.created_at: datetime = datetime.now(timezone.utc)
        
    def is_usable(self) -> bool:
        """Determines if this session can still be used."""
        if self.usage_count >= self.max_usage_count:
            return False
        if self.error_score >= self.max_error_score:
            return False
        return True
        
    def mark_good(self) -> None:
        """Decreases the error score after a successful request."""
        self.usage_count += 1
        self.error_score = max(0.0, self.error_score - self.error_score_decrement)
        
    def mark_bad(self) -> None:
        """Increases error score after a failure (e.g., 500 error or exception)."""
        self.usage_count += 1
        self.error_score += 1.0
        
    def retire(self) -> None:
        """Force retires the session immediately (e.g., on 403 Forbidden)."""
        self.error_score = self.max_error_score

class SessionPool:
    """Manages rotation, creation, and retirement of a pool of sessions."""
    
    def __init__(self, max_pool_size: int = 100):
        self.max_pool_size: int = max_pool_size
        self._sessions: List[Session] = []
        
    def _fill_sessions_to_max(self) -> None:
        """Ensures the pool is full of fresh sessions."""
        while len(self._sessions) < self.max_pool_size:
            self._sessions.append(Session())
            
    def _remove_retired_sessions(self) -> None:
        """Cleans up sessions that are no longer usable."""
        self._sessions = [s for s in self._sessions if s.is_usable()]
        
    def get_session(self) -> Session:
        """Retrieves a random usable session, creating new ones if necessary."""
        self._remove_retired_sessions()
        self._fill_sessions_to_max()
        
        # Pick a random session to spread load
        return random.choice(self._sessions)
        
    def get_session_count(self) -> int:
        return len(self._sessions)
