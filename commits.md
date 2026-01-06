## DuckDuckGo Search Microservice

Added a DuckDuckGo search microservice as the first component of the Searqon pipeline. It accepts search queries via a REST endpoint, performs live DuckDuckGo searches, and parses HTML responses using Cheerio. Results are normalized into a simple JSON format with titles, URLs, and short snippets. The service is scoped strictly to search previews and is designed to plug into future crawler and extractor services.

## Reddit Search Microservice

Added a Reddit search microservice to collect discussion-based signals for Searqon. It accepts Reddit-style queries and fetches results from Reddit’s public JSON search endpoint. Responses are normalized with post metadata such as title, subreddit, score, timestamp, and permalink. The service focuses only on discovery and opinion signals, not full comment extraction.

## Wikipedia Extractor Microservice

Added a Wikipedia extractor microservice to retrieve full article content for URLs discovered during search. It fetches Wikipedia HTML pages and uses Cheerio to extract the main article body while ignoring non-content elements. The service returns clean, structured content and is scoped solely to extraction, enabling future processing such as sectioning and synthesis.

## GitHub Search with README Support

Updated the GitHub search service to fetch the top 10 matching repositories for a query and include each repository’s `README.md` in the response. Results now return basic repo metadata along with raw README content, providing clearer project context while keeping the service search-focused.
