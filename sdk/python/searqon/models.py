"""
Pydantic data models for Searqon API responses.
"""

from datetime import datetime
from typing import Any, Dict, List, Optional
from pydantic import BaseModel, ConfigDict, Field


class Heading(BaseModel):
    model_config = ConfigDict(extra="ignore")
    level: int = 1
    text: str = ""
    anchor: str = ""


class DocumentMetadata(BaseModel):
    model_config = ConfigDict(extra="ignore")
    title: Optional[str] = None
    canonical_url: Optional[str] = None
    description: Optional[str] = None
    author: Optional[str] = None
    published_at: Optional[datetime] = None
    language: Optional[str] = None
    outbound_links: List[str] = Field(default_factory=list)
    structured: Optional[Any] = None


class SearchResult(BaseModel):
    model_config = ConfigDict(extra="ignore")
    title: str = ""
    url: str = ""
    snippet: str = ""
    source: str = ""
    content: Optional[str] = None
    markdown: Optional[str] = None
    scraped: bool = False
    scrape_error: Optional[str] = None
    score: float = 0.0
    metadata: Optional[DocumentMetadata] = None
    token_estimate: int = 0
    headings: List[Heading] = Field(default_factory=list)
    outline: Optional[str] = None
    is_truncated: bool = False
    content_hash: Optional[str] = None
    fetched_at: Optional[str] = None


class SearchResponse(BaseModel):
    model_config = ConfigDict(extra="ignore")
    query: str
    results: List[SearchResult] = Field(default_factory=list)
    total: int = 0
    duration: int = 0
    provider: str = ""
    summary: Optional[str] = None


class ScrapedImage(BaseModel):
    model_config = ConfigDict(extra="ignore")
    url: str
    alt: Optional[str] = None


class ChunkMetadata(BaseModel):
    model_config = ConfigDict(extra="ignore")
    source_url: Optional[str] = None
    source_title: Optional[str] = None
    chunk_index: int = 0
    scraped_at: Optional[str] = None


class Chunk(BaseModel):
    model_config = ConfigDict(extra="ignore")
    index: int = 0
    text: str = ""
    token_count: int = 0
    word_count: int = 0
    bm25_score: Optional[float] = None
    metadata: Optional[ChunkMetadata] = None


class ScrapeResult(BaseModel):
    model_config = ConfigDict(extra="ignore", populate_by_name=True)
    title: str = ""
    content: str = ""
    markdown: Optional[str] = None
    url: str = ""
    word_count: int = Field(default=0, alias="wordCount")
    start_time: Optional[str] = Field(default=None, alias="startTime")
    end_time: Optional[str] = Field(default=None, alias="endTime")
    duration: int = 0
    error: Optional[str] = None
    canonical_url: Optional[str] = Field(default=None, alias="canonicalUrl")
    domain: str = ""
    description: Optional[str] = None
    author: Optional[str] = None
    published_at: Optional[datetime] = Field(default=None, alias="publishedAt")
    language: Optional[str] = None
    outbound_links: List[str] = Field(default_factory=list, alias="outboundLinks")
    status_code: Optional[int] = Field(default=None, alias="statusCode")
    content_type: Optional[str] = Field(default=None, alias="contentType")
    scraped: bool = False
    extraction_method: Optional[str] = Field(default=None, alias="extractionMethod")
    fetch_duration_ms: Optional[int] = Field(default=None, alias="fetchDurationMs")
    images: List[ScrapedImage] = Field(default_factory=list)
    chunks: List[Chunk] = Field(default_factory=list)
    render_method: Optional[str] = None
    escalation_tier: Optional[str] = None
    bot_detected: bool = False
    scraped_at: Optional[str] = None
    cached: bool = False
    content_hash: Optional[str] = None
    content_changed: bool = False
    metadata: Optional[DocumentMetadata] = None
    structured_data: Optional[Any] = None
    token_estimate: int = 0
    headings: List[Heading] = Field(default_factory=list)
    outline: Optional[str] = None
    is_truncated: bool = False


class MapLink(BaseModel):
    model_config = ConfigDict(extra="ignore")
    url: str
    title: Optional[str] = None


class MapResult(BaseModel):
    model_config = ConfigDict(extra="ignore", populate_by_name=True)
    source_url: str = Field(default="", alias="sourceUrl")
    links: List[MapLink] = Field(default_factory=list)
    count: int = 0
    duration: int = 0
    error: Optional[str] = None


class CrawlResult(BaseModel):
    model_config = ConfigDict(extra="ignore", populate_by_name=True)
    source_url: str = Field(default="", alias="sourceUrl")
    pages: List[ScrapeResult] = Field(default_factory=list)
    total: int = 0
    duration: int = 0
    error: Optional[str] = None


class PipelineSource(BaseModel):
    model_config = ConfigDict(extra="ignore")
    url: str = ""
    title: str = ""
    render_method: Optional[str] = None
    cached: bool = False
    chunks: List[Chunk] = Field(default_factory=list)


class PipelineResponse(BaseModel):
    model_config = ConfigDict(extra="ignore")
    query: str
    fetched_at: str = ""
    sources: List[PipelineSource] = Field(default_factory=list)
    total_chunks: int = 0
    duration_ms: int = 0
    context: Optional[str] = None
