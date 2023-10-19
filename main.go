package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

type ScrapedData struct {
	Text  []string `json:"text"`
	Links []string `json:"links"`
}

var (
	cacheLock   sync.RWMutex
	pageCache   = make(map[string]ScrapedData)
	visitedURLs = make(map[string]struct{})
)

func main() {

	http.Handle(
		"/static/",
		http.StripPrefix(
			"/static/",
			http.FileServer(http.Dir("static")),
		),
	)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" {
			http.ServeFile(w, r, "index.html")
		}
	})

	http.HandleFunc("/crawl", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			r.ParseForm()
			url := r.PostFormValue("url")

			if url == "" {
				http.Error(w, "URL is required", http.StatusBadRequest)
				return
			}

			exists, err := checkURLExistenceWithRetries(url, 3, 5) // URL, Max Retries, and Retry Interval (in seconds)

			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}

			if exists {

				// Check if the URL is in the cache and not expired
				cachedData, cacheExists := getFromCache(url)

				if cacheExists {
					// Serve the cached page if available
					fmt.Println("Serving from cache")
					serveCachedPage(w, cachedData)
				} else {

					// Crawl the web page and scrape data and links
					data := crawlWebPage(url)

					for i, text := range data.Text {
						data.Text[i] = strings.TrimSpace(text)
					}

					// Cache the scraped data
					cachePage(url, data)

					// Return the scraped data as JSON
					jsonResponse, err := json.Marshal(data)
					if err != nil {
						http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
						return
					}

					w.Header().Set("Content-Type", "application/json")

					w.Write(jsonResponse)
				}

			} else {
				http.Error(w, "Web page not found", http.StatusNotFound)
			}
		}
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server is running on port %s...\n", port)
	http.ListenAndServe(":"+port, nil)
}

func checkURLExistenceWithRetries(url string, maxRetries int, retryInterval int) (bool, error) {
	var exists bool
	var err error

	for i := 0; i < maxRetries; i++ {
		fmt.Printf("Checking of URL existence, attempt no. %v...\n", i)
		exists, err = checkURLExistence(url)

		if err == nil && exists {
			// URL exists, break out of the loop
			fmt.Printf("URL found...Retrieving data...\n")
			break
		}

		// Wait before retrying
		fmt.Printf("Retrying to fetch again...\n")
		time.Sleep(time.Duration(retryInterval) * time.Second)
	}
	if !exists {
		fmt.Printf("Out of no. of retries\n")
	}
	return exists, err
}

func checkURLExistence(url string) (bool, error) {
	// Create an HTTP client with a timeout to prevent indefinite blocking
	client := &http.Client{Timeout: 10 * time.Second}

	// Send a HEAD request to check the URL
	resp, err := client.Head(url)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	// Check the HTTP status code
	if resp.StatusCode >= 200 && resp.StatusCode < 400 {
		return true, nil
	}
	fmt.Printf("Cannot fetch URl at this time\n")
	return false, nil
}

func isVisited(url string) bool {
	_, visited := visitedURLs[url]
	return visited
}

func crawlWebPage(Weburl string) ScrapedData {
	c := colly.NewCollector()

	var data ScrapedData
	// URL to start scraping
	startURL := Weburl

	baseURL, _ := url.Parse(Weburl)

	visited := make(map[string]struct{})
	var mutex sync.Mutex

	// Set up callbacks for different data types
	c.OnHTML("p", func(e *colly.HTMLElement) {
		// Extract and print text data
		// fmt.Printf("Text: %s\n", e.Text)

		// Extract and add text data to the result
		data.Text = append(data.Text, e.Text)
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		// Extract and print links
		link := e.Request.AbsoluteURL(e.Attr("href"))
		// fmt.Printf("Link: %s\n", link)

		// Extract and add text data to the result
		data.Text = append(data.Text, e.Text)
		linkURL, err := url.Parse(link)
		if err != nil {
			// Handle the error
			// log.Printf("Error parsing URL %s: %v", link, err)
			return
		}

		if linkURL.Host == baseURL.Host && !isVisited(link) {
			mutex.Lock()
			data.Links = append(data.Links, link)
			visited[link] = struct{}{}
			mutex.Unlock()

		}
	})

	// Error handling
	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Request URL: %s\nError: %v", r.Request.URL, err)
	})

	// Start scraping from the initial URL
	err := c.Visit(startURL)
	if err != nil {
		log.Printf("Error visiting %s: %v", startURL, err)
	}

	return data
}

func cachePage(url string, data ScrapedData) {
	cacheLock.Lock()
	defer cacheLock.Unlock()
	pageCache[url] = data
}

func getFromCache(url string) (ScrapedData, bool) {
	cacheLock.RLock()
	defer cacheLock.RUnlock()
	cachedData, exists := pageCache[url]
	return cachedData, exists
}

func serveCachedPage(w http.ResponseWriter, data ScrapedData) {
	w.Header().Set("Content-Type", "application/json")
	jsonResponse, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
		return
	}
	w.Write(jsonResponse)
}
