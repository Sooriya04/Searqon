"""
Searqon Python SDK
Official client library for Searqon Web Intelligence Engine.
"""

from .client import AsyncSearqon, Searqon
from .exceptions import (
    SearqonAPIError,
    SearqonBlockedError,
    SearqonConnectionError,
    SearqonError,
    SearqonRateLimitError,
)
from .models import (
    Chunk,
    ChunkMetadata,
    CrawlResult,
    DocumentMetadata,
    Heading,
    MapLink,
    MapResult,
    PipelineResponse,
    PipelineSource,
    ScrapedImage,
    ScrapeResult,
    SearchResponse,
    SearchResult,
)

__version__ = "0.1.0"

__all__ = [
    "Searqon",
    "AsyncSearqon",
    "SearchResponse",
    "SearchResult",
    "ScrapeResult",
    "ScrapedImage",
    "Chunk",
    "ChunkMetadata",
    "DocumentMetadata",
    "Heading",
    "PipelineResponse",
    "PipelineSource",
    "MapResult",
    "MapLink",
    "CrawlResult",
    "SearqonError",
    "SearqonAPIError",
    "SearqonConnectionError",
    "SearqonRateLimitError",
    "SearqonBlockedError",
]
