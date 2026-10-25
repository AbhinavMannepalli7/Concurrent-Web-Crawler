package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/html"
)

const (
	MAX_PAGES     = 20
	NUM_CONSUMERS = 4
	NUM_PRODUCERS = 2
)

type pageData struct {
	pageTitle      string
	HTTPStatusCode int
	outgoingLinks  int
	ResponseSize   int
	Latency        int
}

type PageJob struct {
	URL  string
	Doc  *html.Node
	Data *pageData
}

type FetchResult struct {
	URL  string
	Doc  *html.Node
	Data *pageData
	Err  error
}

type DiscoveryResult struct {
	URL   string
	Links []string
	Title string
}

func normHost(h string) string {
	return strings.TrimPrefix(strings.ToLower(h), "www.")
}

// standardises url so easier to compare
func normalize(base *url.URL, href string) (string, error) {
	u, err := url.Parse(href)
	if err != nil {
		return "", err
	}

	u = base.ResolveReference(u)
	u.Fragment = ""

	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("unsupported scheme")
	}

	if normHost(u.Hostname()) != normHost(base.Hostname()) {
		return "", errors.New("different host")
	}

	return u.String(), nil
}

func processPage(cur string, client *http.Client, data *pageData) (*html.Node, error) {
	req, err := http.NewRequest(http.MethodGet, cur, nil)
	if err != nil {
		return nil, err
	}

	// required as some pages don't accept generic user agents
	req.Header.Set("User-Agent", "concCrawler/0.1 (you@example.com)")

	start := time.Now()
	res, err := client.Do(req)
	data.Latency = int(time.Since(start))

	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	data.HTTPStatusCode = res.StatusCode

	if res.StatusCode > 299 {
		return nil, fmt.Errorf("HTTP status %d", res.StatusCode)
	}

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	data.ResponseSize = len(body)

	return html.Parse(bytes.NewReader(body))
}

// Consumers take URLs, fetch pages, and send results to the coordinator
func consumer(client *http.Client, urlQueue <-chan string, results chan<- FetchResult) {
	for cur := range urlQueue {
		data := &pageData{}
		doc, err := processPage(cur, client, data)

		results <- FetchResult{cur, doc, data, err}
	}
}

// Producers take parsed documents and extract links
// They report links to the coordinator rather than modifying visited
func producer(docQueue <-chan PageJob, discovered chan<- DiscoveryResult) {
	for page := range docQueue {
		result := DiscoveryResult{URL: page.URL}

		base, err := url.Parse(page.URL)
		if err != nil || page.Doc == nil {
			discovered <- result
			continue
		}

		var walk func(*html.Node)
		walk = func(n *html.Node) {
			if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
				result.Title = n.FirstChild.Data
			}

			if n.Type == html.ElementNode && n.Data == "a" {
				for _, attr := range n.Attr {
					if attr.Key != "href" {
						continue
					}

					link, err := normalize(base, attr.Val)
					if err == nil {
						result.Links = append(result.Links, link)
					}
				}
			}

			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
		}

		walk(page.Doc)
		discovered <- result
	}
}

func crawl(rootURL string, client *http.Client) {
	// Buffers are bounded by the maximum number of pages
	urlQueue := make(chan string, MAX_PAGES)
	docQueue := make(chan PageJob, MAX_PAGES)
	results := make(chan FetchResult, MAX_PAGES)
	discovered := make(chan DiscoveryResult, MAX_PAGES)

	var consumerWG sync.WaitGroup
	var producerWG sync.WaitGroup

	for range NUM_CONSUMERS {
		consumerWG.Add(1)
		go func() {
			defer consumerWG.Done()
			consumer(client, urlQueue, results)
		}()
	}

	for range NUM_PRODUCERS {
		producerWG.Add(1)
		go func() {
			defer producerWG.Done()
			producer(docQueue, discovered)
		}()
	}

	// Only crawl() modifies the visited map and pending counter
	visited := map[string]*pageData{rootURL: {}}

	seen := 1
	pending := 1
	urlQueue <- rootURL

	for pending > 0 {
		select {
		case result := <-results:
			if result.Err != nil {
				fmt.Printf("Failed: %s: %v\n", result.URL, result.Err)
				pending--
				continue
			}

			// Keep this page outstanding until link discovery finishes
			docQueue <- PageJob{result.URL, result.Doc, result.Data}

			visited[result.URL] = result.Data

		case result := <-discovered:
			data := visited[result.URL]
			data.pageTitle = result.Title

			for _, link := range result.Links {
				if _, exists := visited[link]; exists {
					continue
				}

				if seen >= MAX_PAGES {
					break
				}

				// Register new work before completing the current page
				visited[link] = &pageData{}
				seen++
				pending++
				data.outgoingLinks++

				urlQueue <- link
			}

			// This page has finished link discovery
			pending--
		}
	}

	// All pages have finished, including link discovery
	close(urlQueue)
	consumerWG.Wait()

	close(docQueue)
	producerWG.Wait()

	fmt.Println("\nCrawl results:")
	for link, data := range visited {
		fmt.Printf("%s | title=%q status=%d links=%d size=%d latency=%s\n", link, data.pageTitle, data.HTTPStatusCode, data.outgoingLinks, data.ResponseSize, time.Duration(data.Latency))
	}

	fmt.Printf("\nFinished crawling: %d pages scheduled\n", seen)
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: crawler <root-url>")
		os.Exit(1)
	}

	client := &http.Client{Timeout: 10 * time.Second}

	crawl(os.Args[1], client)
}
