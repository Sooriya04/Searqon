from typing import Callable, Coroutine, Any, Dict
import logging

from .models import BasicCrawlingContext

# Use typing aliases for handler functions
HandlerCallable = Callable[[Any], Coroutine[Any, Any, None]]

logger = logging.getLogger(__name__)

class Router:
    """
    Routes requests to specific handlers based on user_data['label'].
    Identical interface to the original Crawlee.
    """
    def __init__(self) -> None:
        self._handlers: Dict[str, HandlerCallable] = {}
        self._default_handler: HandlerCallable | None = None

    def default_handler(self, handler: HandlerCallable) -> HandlerCallable:
        """Decorator mapping the default logic when no label matches."""
        self._default_handler = handler
        return handler

    def add_handler(self, label: str) -> Callable[[HandlerCallable], HandlerCallable]:
        """Decorator mapping a specific label string to a handler."""
        def decorator(handler: HandlerCallable) -> HandlerCallable:
            self._handlers[label] = handler
            return handler
        return decorator

    async def invoke(self, context: BasicCrawlingContext) -> None:
        """Executes the appropriate handler according to the request label."""
        label = context.request.user_data.get('label')
        
        if label and label in self._handlers:
            await self._handlers[label](context)
        elif self._default_handler:
            await self._default_handler(context)
        else:
            logger.warning(f"No handler defined for label '{label}' on URL {context.request.url}.")
