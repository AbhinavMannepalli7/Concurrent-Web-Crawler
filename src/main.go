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
	"time"

	"golang.org/x/net/html"
)

const MAX_DEPTH = 3
const MAX_PAGES = 20

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

type pageData struct {
	pageTitle      string
	HTTPStatusCode int
	outgoingLinks  int
	ResponseSize   int
	Latency        int
}

func main() {
	// https://example.com
	root_url := os.Args[1]
	root := Node{root_url, nil}
	q := Queue{1, &root, &root}
	visited := map[string]*pageData{root_url: {}}
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	seen := 0
	for q.size > 0 && seen < MAX_PAGES {
		lenn := q.size
		for range lenn {
			if seen >= MAX_PAGES {
				break
			}
			cur, _ := q.pop()

			req, err := http.NewRequest(http.MethodGet, cur, nil)
			if err != nil {
				continue
			}

			// some websites rate limit generic User Agents
			req.Header.Set("User-Agent", "MyLearningCrawler/0.1 (you@example.com)")
			start := time.Now()
			res, err := client.Do(req)
			visited[cur].Latency = int(time.Since(start))
			if err != nil {
				// timed out or failed, skip this url
				continue
			}

			if res.StatusCode > 299 {
				res.Body.Close()
				continue
			}

			seen++

			visited[cur].HTTPStatusCode = res.StatusCode

			body, err := io.ReadAll(res.Body)
			res.Body.Close()
			if err != nil {
				continue
			}
			visited[cur].ResponseSize = len(body)

			doc, err := html.Parse(bytes.NewReader(body))

			base, _ := url.Parse(cur)

			var visit func(n *html.Node)
			visit = func(n *html.Node) {
				if n.Type == html.ElementNode && n.Data == "title" && n.FirstChild != nil {
					visited[cur].pageTitle = n.FirstChild.Data
				}
				if n.Type == html.ElementNode && n.Data == "a" {
					for _, attr := range n.Attr {
						if attr.Key == "href" {
							link, err := normalize(base, attr.Val)
							if err != nil {
								continue
							}
							if _, seen := visited[link]; !seen {
								q.push(link)
								visited[cur].outgoingLinks++
								visited[link] = &pageData{}
							}
						}
					}
				}
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					visit(c)
				}
			}
			visit(doc)
			fmt.Println(cur, visited[cur])
		}
	}
	fmt.Println("Finished Crawling")
	return
}
