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

const MAX_PAGES = 20

var mu sync.Mutex

type pageData struct {
	pageTitle      string
	HTTPStatusCode int
	outgoingLinks  int
	ResponseSize   int
	Latency        int
}

type PageJob struct {
	URL string
	Doc *html.Node
}

func normHost(h string) string {
	return strings.TrimPrefix(strings.ToLower(h), "www.")
}

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

func processPage(cur string, client *http.Client, visited map[string]*pageData) (*html.Node, error) {
	req, err := http.NewRequest(http.MethodGet, cur, nil)
	if err != nil {
		return nil, err
	}

	// Some websites rate limit generic User Agents.
	req.Header.Set("User-Agent", "MyLearningCrawler/0.1 (you@example.com)")

	start := time.Now()
	res, err := client.Do(req)
	visited[cur].Latency = int(time.Since(start))

	if err != nil {
		// Timed out or failed; skip this URL.
		return nil, err
	}

	if res.StatusCode > 299 {
		res.Body.Close()
		return nil, err
	}

	visited[cur].HTTPStatusCode = res.StatusCode

	body, err := io.ReadAll(res.Body)
	res.Body.Close()

	if err != nil {
		return nil, err
	}

	visited[cur].ResponseSize = len(body)

	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}

	return doc, nil
}

func visit(
	n *html.Node, cur string, visited map[string]*pageData, q *Queue) {
	base, _ := url.Parse(cur)

	// get page title
	if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
		visited[cur].pageTitle = n.FirstChild.Data
	}

	// discover and enqueue links.
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key != "href" {
				continue
			}

			link, err := normalize(base, attr.Val)
			if err != nil {
				continue
			}

			mu.Lock()
			if _, seen := visited[link]; !seen {
				q.push(link)
				visited[cur].outgoingLinks++
				visited[link] = &pageData{}
			}
			mu.Unlock()
		}
	}

	// Recursively visit child nodes.
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		visit(c, cur, visited, q)
	}
}

func crawl(rootURL string, client *http.Client) {
	root := Node{rootURL, nil}
	q := Queue{1, &root, &root}
	visited := map[string]*pageData{rootURL: {}}

	seen := 0

	for q.size > 0 && seen < MAX_PAGES {
		cur, _ := q.pop()

		doc, err := processPage(cur, client, visited)
		if err != nil {
			// Do something.
		}

		seen++

		go visit(doc, cur, visited, &q)

		fmt.Println(cur, visited[cur])
	}

	fmt.Println("Finished Crawling")
}

func producer(urlQueue chan string, docQueue chan PageJob) {
	page := <-docQueue
	cur := page.URL
	n := page.Doc

	base, _ := url.Parse(cur)

	/*
		// get page title
		if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
			visited[cur].pageTitle = n.FirstChild.Data
		}

	*/

	// discover and enqueue links.
	if n.Type == html.ElementNode && n.Data == "a" {
		for _, attr := range n.Attr {
			if attr.Key != "href" {
				continue
			}

			link, err := normalize(base, attr.Val)
			if err != nil {
				continue
			}

			urlQueue <- link

			/*
				mu.Lock()
				if _, seen := visited[link]; !seen {
					q.push(link)
					visited[cur].outgoingLinks++
					visited[link] = &pageData{}
				}
				mu.Unlock()
			*/
		}
	}
}

func consumer(client *http.Client) {
	cur := <-urlQueue

	req, _ := http.NewRequest(http.MethodGet, cur, nil)

	// Some websites rate limit generic User Agents.
	req.Header.Set("User-Agent", "MyLearningCrawler/0.1 (you@example.com)")

	//start := time.Now()
	res, _ := client.Do(req)
	//visited[cur].Latency = int(time.Since(start))

	if res.StatusCode > 299 {
		res.Body.Close()

	}

	//visited[cur].HTTPStatusCode = res.StatusCode

	body, _ := io.ReadAll(res.Body)
	res.Body.Close()

	//visited[cur].ResponseSize = len(body)

	doc, _ := html.Parse(bytes.NewReader(body))

	docQueue <- PageJob{cur, doc}
}

func main() {
	rootURL := os.Args[1]

	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	urlQueue := make(chan string, 100)
	docQueue := make(chan PageJob, 100)

	urlQueue <- rootURL

}
