"""
Comprehensive unit tests for Searqon Python SDK.
"""

import pytest
import httpx
from unittest.mock import MagicMock, AsyncMock, patch

from searqon import (
    Searqon,
    AsyncSearqon,
    SearchResponse,
    ScrapeResult,
    PipelineResponse,
    MapResult,
    CrawlResult,
)
from searqon.exceptions import (
    SearqonAPIError,
    SearqonBlockedError,
    SearqonConnectionError,
    SearqonRateLimitError,
)


def test_client_init_and_context_manager():
    client = Searqon(base_url="http://localhost:7493/")
    assert client.base_url == "http://localhost:7493"
    with client:
        pass


def test_search_success():
    mock_payload = {
        "query": "artificial intelligence",
        "total": 1,
        "duration": 120,
        "provider": "multi",
        "results": [
            {
                "title": "AI Overview",
                "url": "https://example.com/ai",
                "snippet": "Intro to AI",
                "source": "searxng",
                "markdown": "# AI\nOverview of AI...",
                "scraped": True,
                "score": 0.95,
            }
        ],
    }

    mock_resp = httpx.Response(200, json=mock_payload)
    with patch.object(httpx.Client, "post", return_value=mock_resp) as mock_post:
        client = Searqon("http://localhost:7493")
        resp = client.search("artificial intelligence", limit=5, scrape=True)

        assert isinstance(resp, SearchResponse)
        assert resp.total == 1
        assert resp.query == "artificial intelligence"
        assert len(resp.results) == 1
        assert resp.results[0].title == "AI Overview"
        assert resp.results[0].scraped is True
        assert resp.results[0].score == 0.95

        # Check call payload
        call_kwargs = mock_post.call_args[1]
        assert call_kwargs["json"]["query"] == "artificial intelligence"
        assert call_kwargs["json"]["limit"] == 5
        assert call_kwargs["json"]["scrape"] is True


def test_scrape_success():
    mock_payload = {
        "title": "Hacker News",
        "url": "https://news.ycombinator.com",
        "content": "HN plain text",
        "markdown": "# Hacker News",
        "wordCount": 450,
        "canonicalUrl": "https://news.ycombinator.com",
        "domain": "ycombinator.com",
        "scraped": True,
        "render_method": "fast_http",
        "bot_detected": False,
    }

    mock_resp = httpx.Response(200, json=mock_payload)
    with patch.object(httpx.Client, "post", return_value=mock_resp):
        client = Searqon()
        res = client.scrape("https://news.ycombinator.com", max_words=500)

        assert isinstance(res, ScrapeResult)
        assert res.title == "Hacker News"
        assert res.word_count == 450
        assert res.canonical_url == "https://news.ycombinator.com"
        assert res.render_method == "fast_http"
        assert res.bot_detected is False


def test_pipeline_success():
    mock_payload = {
        "query": "rag pipelines",
        "fetched_at": "2026-11-26T12:00:00Z",
        "total_chunks": 3,
        "duration_ms": 350,
        "sources": [
            {
                "url": "https://example.com/rag",
                "title": "RAG Guide",
                "cached": False,
                "chunks": [
                    {
                        "index": 0,
                        "text": "RAG chunk 1",
                        "token_count": 25,
                        "word_count": 15,
                        "bm25_score": 3.8,
                        "metadata": {"chunk_index": 0, "source_title": "RAG Guide"},
                    }
                ],
            }
        ],
    }

    mock_resp = httpx.Response(200, json=mock_payload)
    with patch.object(httpx.Client, "post", return_value=mock_resp):
        client = Searqon()
        pipeline = client.pipeline("rag pipelines", max_sources=3)

        assert isinstance(pipeline, PipelineResponse)
        assert pipeline.total_chunks == 3
        assert len(pipeline.sources) == 1
        assert pipeline.sources[0].chunks[0].bm25_score == 3.8


def test_map_and_crawl():
    map_payload = {
        "sourceUrl": "https://example.com",
        "links": [{"url": "https://example.com/about", "title": "About"}],
        "count": 1,
        "duration": 50,
    }
    with patch.object(httpx.Client, "post", return_value=httpx.Response(200, json=map_payload)):
        client = Searqon()
        m = client.map("https://example.com")
        assert isinstance(m, MapResult)
        assert m.count == 1
        assert m.links[0].url == "https://example.com/about"


def test_rate_limit_error():
    mock_resp = httpx.Response(429, json={"error": "Too many requests", "retryable": True})
    with patch.object(httpx.Client, "post", return_value=mock_resp):
        client = Searqon()
        with pytest.raises(SearqonRateLimitError) as exc_info:
            client.search("rate limited query")
        assert exc_info.value.status_code == 429
        assert exc_info.value.retryable is True


def test_anti_bot_blocked_error():
    mock_resp = httpx.Response(403, json={"error": "CLOUDFLARE_BLOCKED", "message": "Access denied"})
    with patch.object(httpx.Client, "post", return_value=mock_resp):
        client = Searqon()
        with pytest.raises(SearqonBlockedError) as exc_info:
            client.scrape("https://protected.com")
        assert exc_info.value.status_code == 403


def test_connection_error():
    with patch.object(httpx.Client, "post", side_effect=httpx.ConnectError("Connection refused")):
        client = Searqon("http://localhost:9999")
        with pytest.raises(SearqonConnectionError):
            client.search("query")


@pytest.mark.asyncio
async def test_async_client_search_and_context():
    mock_payload = {
        "query": "async query",
        "total": 1,
        "duration": 80,
        "provider": "multi",
        "results": [{"title": "Async Title", "url": "https://async.org", "snippet": "Async..."}],
    }
    mock_resp = httpx.Response(200, json=mock_payload)

    with patch.object(httpx.AsyncClient, "post", return_value=mock_resp):
        async with AsyncSearqon("http://localhost:7493") as client:
            resp = await client.search("async query")
            assert isinstance(resp, SearchResponse)
            assert resp.query == "async query"
            assert resp.results[0].title == "Async Title"


def test_sync_client_unwraps_response_envelope():
    mock_inner_payload = {
        "query": "wrapped query",
        "total": 1,
        "duration": 50,
        "provider": "multi",
        "results": [
            {
                "title": "Wrapped Title",
                "url": "https://wrapped.org",
                "snippet": "Wrapped Snippet",
                "source": "searxng",
                "scraped": False,
            }
        ],
    }
    wrapped_resp = httpx.Response(200, json={"success": True, "data": mock_inner_payload})

    with patch.object(httpx.Client, "post", return_value=wrapped_resp):
        client = Searqon()
        resp = client.search("wrapped query")
        assert isinstance(resp, SearchResponse)
        assert resp.query == "wrapped query"
        assert resp.results[0].title == "Wrapped Title"


@pytest.mark.asyncio
async def test_async_client_unwraps_response_envelope():
    mock_inner_payload = {
        "title": "Async Scrape",
        "url": "https://async-wrapped.org",
        "content": "Async wrapped text",
        "markdown": "# Async wrapped",
        "wordCount": 100,
        "canonicalUrl": "https://async-wrapped.org",
        "domain": "async-wrapped.org",
        "scraped": True,
        "render_method": "go",
        "bot_detected": False,
    }
    wrapped_resp = httpx.Response(200, json={"success": True, "data": mock_inner_payload})

    with patch.object(httpx.AsyncClient, "post", return_value=wrapped_resp):
        async with AsyncSearqon() as client:
            res = await client.scrape("https://async-wrapped.org")
            assert isinstance(res, ScrapeResult)
            assert res.title == "Async Scrape"
            assert res.word_count == 100


def test_searxng_url_client_level():
    mock_payload = {
        "query": "searxng client test",
        "total": 1,
        "results": [{"title": "R1", "url": "https://example.com", "source": "searxng"}],
    }
    mock_resp = httpx.Response(200, json={"success": True, "data": mock_payload})
    with patch.object(httpx.Client, "post", return_value=mock_resp) as mock_post:
        client = Searqon("http://localhost:7493", searxng_url="http://custom-searxng:8080")
        assert client.searxng_url == "http://custom-searxng:8080"

        client.search("searxng client test")
        call_kwargs = mock_post.call_args[1]
        assert call_kwargs["json"]["searxng_url"] == "http://custom-searxng:8080"
        assert call_kwargs["headers"]["X-Searxng-URL"] == "http://custom-searxng:8080"


def test_searxng_url_method_override():
    mock_payload = {
        "query": "searxng override test",
        "total": 1,
        "results": [{"title": "R1", "url": "https://example.com", "source": "searxng"}],
    }
    mock_resp = httpx.Response(200, json={"success": True, "data": mock_payload})
    with patch.object(httpx.Client, "post", return_value=mock_resp) as mock_post:
        client = Searqon("http://localhost:7493", searxng_url="http://default-searxng:8080")
        client.search("searxng override test", searxng_url="http://override-searxng:9090")
        call_kwargs = mock_post.call_args[1]
        assert call_kwargs["json"]["searxng_url"] == "http://override-searxng:9090"
        assert call_kwargs["headers"]["X-Searxng-URL"] == "http://override-searxng:9090"


def test_pipeline_searxng_url():
    mock_payload = {
        "query": "pipeline searxng test",
        "sources": [],
        "chunks": [],
        "totalSources": 0,
        "totalChunks": 0,
        "duration": 50,
    }
    mock_resp = httpx.Response(200, json={"success": True, "data": mock_payload})
    with patch.object(httpx.Client, "post", return_value=mock_resp) as mock_post:
        client = Searqon("http://localhost:7493")
        client.pipeline("pipeline searxng test", searxng_url="http://pipe-searxng:8080")
        call_kwargs = mock_post.call_args[1]
        assert call_kwargs["json"]["searxng_url"] == "http://pipe-searxng:8080"
        assert call_kwargs["headers"]["X-Searxng-URL"] == "http://pipe-searxng:8080"


@pytest.mark.asyncio
async def test_async_searxng_url():
    mock_payload = {
        "query": "async searxng test",
        "total": 1,
        "results": [{"title": "R1", "url": "https://example.com", "source": "searxng"}],
    }
    mock_resp = httpx.Response(200, json={"success": True, "data": mock_payload})
    with patch.object(httpx.AsyncClient, "post", new_callable=AsyncMock, return_value=mock_resp) as mock_post:
        client = AsyncSearqon(searxng_url="http://async-searxng:8080")
        await client.search("async searxng test")
        call_kwargs = mock_post.call_args[1]
        assert call_kwargs["json"]["searxng_url"] == "http://async-searxng:8080"
        assert call_kwargs["headers"]["X-Searxng-URL"] == "http://async-searxng:8080"


def test_health_unwraps_data_envelope():
    mock_payload = {
        "success": True,
        "data": {
            "status": "ok",
            "engine": "searqon",
            "endpoints": ["/search", "/scrape", "/pdf", "/health"],
        },
    }
    mock_resp = httpx.Response(200, json=mock_payload)
    with patch.object(httpx.Client, "get", return_value=mock_resp):
        client = Searqon()
        health = client.health()
        assert health["status"] == "ok"
        assert "success" not in health
        assert "/pdf" in health["endpoints"]


@pytest.mark.asyncio
async def test_async_health_unwraps_data_envelope():
    mock_payload = {
        "success": True,
        "data": {
            "status": "ok",
            "engine": "searqon",
            "endpoints": ["/search", "/scrape", "/pdf", "/health"],
        },
    }
    mock_resp = httpx.Response(200, json=mock_payload)
    with patch.object(httpx.AsyncClient, "get", new_callable=AsyncMock, return_value=mock_resp):
        client = AsyncSearqon()
        health = await client.health()
        assert health["status"] == "ok"
        assert "success" not in health
        assert "/pdf" in health["endpoints"]
