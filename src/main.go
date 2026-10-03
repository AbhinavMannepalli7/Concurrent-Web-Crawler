package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

// make sure urls start with https
func parse_links(body string) []string {
	res := []string{}
	i := 0
	for i < len(body) {
		found := strings.Index(body[i:], "https://")
		if found == -1 {
			break
		}
		start := found + i
		end := start
		for (end < len(body)) && (body[end] != '"') {
			end++
		}
		res = append(res, body[start:end])
		i = end + 1
	}
	return res
}

func main() {
	// https://example.com
	url := os.Args[1]
	root := Node{url, nil}
	q := Queue{1, &root, &root}
	visited := map[string]bool{url: true}
	for q.size > 0 {
		// check status code later
		cur, _ := q.pop()
		fmt.Println(cur)

		res, err := http.Get(cur)
		if err != nil {
			log.Fatal(err)
		}

		body, _ := io.ReadAll(res.Body)
		res.Body.Close()

		links := parse_links(string(body))

		for _, link := range links {
			if !visited[link] {
				q.push(link)
				visited[link] = true
			}
		}
	}
}
