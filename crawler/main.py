import asyncio
import logging
import os

from src import BeautifulSoupCrawler, BeautifulSoupCrawlingContext

# Configure basic logging for visibility
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')

async def main():
    # 1. Initialize our rebuilt Full Architecture Crawler
    # By default, it manages a SessionPool and Autoscaler. 
    # Let's set max_concurrency to 10 for testing.
    crawler = BeautifulSoupCrawler(max_concurrency=10, max_request_retries=2)

    # 2. Define our handler
    @crawler.router.default_handler
    async def process_page(context: BeautifulSoupCrawlingContext) -> None:
        context.log.info(f"Handler processing: {context.request.url}")
        
        # Scrape data safely using the BeautifulSoup object provided by the context pipeline
        title = context.soup.title.string.strip() if context.soup.title else "No Title"
        
        data = {
            "url": context.request.url,
            "title": title,
            "session_used": context.request.session_id, # Demonstrate SessionTracker
            "status": context.response.status
        }
        
        # Push to storage/datasets/default/dataset.jsonl
        await context.push_data(data)

    # 3. Queue multiple requests to demonstrate autoscaler efficiency
    targets = [
        "https://example.com",
        "https://crawlee.dev",
        "https://apify.com",
        "https://httpbin.org/get",
        "https://httpbin.org/html"
    ]
    
    # 4. Start the engine
    await crawler.run(targets)

if __name__ == "__main__":
    dataset_file = "dataset.jsonl"
    if os.path.exists(dataset_file):
        os.remove(dataset_file)
        
    asyncio.run(main())
