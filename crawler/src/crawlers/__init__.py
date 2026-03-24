# For the crawler modules
from .basic_crawler import BasicCrawler
from .http_crawler import AbstractHttpCrawler
from .scrapling_crawler import ScraplingCrawler

__all__ = [
    'BasicCrawler',
    'AbstractHttpCrawler',
    'ScraplingCrawler'
]
