# For the crawler modules
from .basic_crawler import BasicCrawler
from .http_crawler import AbstractHttpCrawler
from .bs_crawler import BeautifulSoupCrawler

__all__ = [
    'BasicCrawler',
    'AbstractHttpCrawler',
    'BeautifulSoupCrawler'
]
