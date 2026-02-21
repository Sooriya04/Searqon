import asyncio
import logging
import uuid
import time
from datetime import datetime, timezone
from aiohttp import web
from src import BeautifulSoupCrawler, BeautifulSoupCrawlingContext, Request

logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger('api_server')

# Global state mapping request IDs to futures
pending_requests = {}
crawler = BeautifulSoupCrawler(max_concurrency=10, max_request_retries=2)

@crawler.router.default_handler
async def process_page(context: BeautifulSoupCrawlingContext) -> None:
    req_id = context.request.user_data.get('req_id')
    if not req_id or req_id not in pending_requests:
        return
        
    title = context.soup.title.string.strip() if context.soup.title else "No Title"
    
    # Mimic ScrapUrl.js element removing
    for s in context.soup(['script', 'style', 'noscript', 'iframe', 'svg', 'nav']):
        s.extract()
        
    body_text = context.soup.get_text(separator='\n')
    clean_body = '\n'.join([line.strip() for line in body_text.split('\n') if line.strip()])
    word_count = len(clean_body.split())
    
    data = {
        "title": title,
        "content": clean_body,
        "url": context.request.url,
        "wordCount": word_count,
        "status": "success"
    }
    
    future = pending_requests[req_id]
    if not future.done():
        future.set_result(data)

async def scrape_handler(request):
    try:
        body = await request.json()
        url = body.get('url')
        if not url:
            return web.json_response({"error": "URL required"}, status=400)
            
        req_id = str(uuid.uuid4())
        future = asyncio.get_event_loop().create_future()
        pending_requests[req_id] = future
        
        start_time_iso = datetime.now(timezone.utc).isoformat()
        start_time_ms = time.time()
        
        # Override unique_key so multiple identical URLs can be queried over time
        # The storage queue deduplicates by unique_key.
        unique_key = f"{url}_{req_id}" 
        
        req = Request(url=url, user_data={'req_id': req_id})
        req.unique_key = unique_key
        
        await crawler.request_queue.add_request(req)
        
        try:
            # Wait up to 15 seconds for the crawler worker to process it
            result = await asyncio.wait_for(future, timeout=15.0)
            
            # Decorate with timing identical to ScrapUrl.js
            end_time_ms = time.time()
            end_time_iso = datetime.now(timezone.utc).isoformat()
            
            report = {
                "title": result["title"],
                "content": result["content"],
                "url": result["url"],
                "wordCount": result["wordCount"],
                "startTime": start_time_iso,
                "endTime": end_time_iso,
                "duration": int((end_time_ms - start_time_ms) * 1000)
            }
            return web.json_response(report)
            
        except asyncio.TimeoutError:
            if not future.done():
                future.cancel()
            return web.json_response({"error": "Timeout or failed to scrape due to bot blocking/network issue"}, status=504)
        finally:
            pending_requests.pop(req_id, None)
            
    except Exception as e:
        logger.error(f"Handler error: {e}")
        return web.json_response({"error": str(e)}, status=500)

async def start_background_tasks(app):
    logger.info("Starting background Crawlee Autoscaler...")
    await crawler.autoscaler.start()

async def cleanup_background_tasks(app):
    logger.info("Stopping background Crawlee Autoscaler...")
    await crawler.autoscaler.stop()

app = web.Application()
app.router.add_post('/scrape', scrape_handler)

app.on_startup.append(start_background_tasks)
app.on_cleanup.append(cleanup_background_tasks)

if __name__ == '__main__':
    web.run_app(app, port=3002)
