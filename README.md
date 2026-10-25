# Concurrent Web Crawler

A web crawler written in Go that explores pages on a website, discovers links, and collects basic metadata about each page.

The project is a learning exercise in **concurrent programming in Go**, with a focus on goroutines, channels, producer–consumer architecture, and coordinating concurrent work.

## Features

- **Concurrent page fetching** using multiple consumer workers.
- **Concurrent link discovery** using multiple producer workers.
- **Two-stage pipeline** connected by Go channels.
- **URL normalization** to resolve relative links and remove fragments.
- **Same-host crawling** to keep the crawler within the starting website's host.
- **URL deduplication** to avoid scheduling the same normalized URL more than once.
- **Page metadata collection**, including HTTP status, response size, latency, page title, and outgoing link count.
- **Configurable crawl limits** to restrict the number of pages processed.

## Architecture

The crawler uses a two-queue producer–consumer pipeline.

```text
                 ┌──────────────────┐
                 │    Root URL      │
                 └────────┬─────────┘
                          │
                          ▼
                 ┌──────────────────┐
                 │     URL Queue    │
                 └────────┬─────────┘
                          │
                          ▼
                 ┌──────────────────┐
                 │ Consumer Workers │
                 │  Fetch HTML      │
                 │  Parse Document  │
                 └────────┬─────────┘
                          │
                          ▼
                 ┌──────────────────┐
                 │   Document Queue │
                 └────────┬─────────┘
                          │
                          ▼
                 ┌──────────────────┐
                 │ Producer Workers │
                 │  Extract Links   │
                 │  Normalize URLs  │
                 └────────┬─────────┘
                          │
                          ▼
                 ┌──────────────────┐
                 │   Coordinator    │
                 │ Deduplicate URLs│
                 │ Track Work       │
                 └────────┬─────────┘
                          │
                          └──────► URL Queue
```

## Metadata collected

The crawler maintains a `pageData` record for each scheduled URL.

| Field | Description |
|---|---|
| `pageTitle` | The page's HTML title |
| `HTTPStatusCode` | HTTP response status code |
| `outgoingLinks` | Number of unique links discovered from the page that were scheduled |
| `ResponseSize` | Response body size in bytes |
| `Latency` | Time taken for the HTTP request, in nanoseconds |

## Configuration

The following constants control the crawl:

```go
const (
    MAX_PAGES     = 20
    NUM_CONSUMERS = 4
    NUM_PRODUCERS = 2
)
```

- `MAX_PAGES`: Maximum number of unique URLs scheduled for crawling, including the starting URL.
- `NUM_CONSUMERS`: Number of concurrent page-fetching workers.
- `NUM_PRODUCERS`: Number of concurrent link-discovery workers.

The HTTP client has a 10-second timeout to prevent requests from waiting indefinitely.
