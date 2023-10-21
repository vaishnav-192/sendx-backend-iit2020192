package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	// "net/url"
	"os"
	// "strings"
	"sync"
	"time"

	"github.com/gocolly/colly/v2"
)

type ScrapedData struct {
	Links []string `json:"links"`
}

var mu sync.Mutex

var Cachedata = make(map[string]cachedata)

type cachedata struct {
	url  string
	data ScrapedData
	Time time.Time
}

var visitedURLs = make(map[string]bool)

func main() {

	http.HandleFunc("/crawl", crawlhandler)
	http.Handle("/", http.FileServer(http.Dir(".")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	fmt.Printf("Server is running on port %s...\n", port)
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal("error in server listening...", err)
	}
}

func crawlhandler(w http.ResponseWriter, r *http.Request) {

	r.ParseForm()
	url := r.PostFormValue("url")
	customerType := r.PostFormValue("customerType")

	fmt.Printf("%s and %s\n", url, customerType)

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

		mu.Lock()
		defer mu.Unlock()

		cache, present := Cachedata[url]
		if present {
			jsonResponse, err := json.Marshal(cache)
			if err != nil {
				http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "application/json")

			w.Write(jsonResponse)
		} else {

			// Crawl the web page and scrape data and links
			data := crawlWebPage(url)

			Cachedata[url] = cachedata{
				url:  url,
				data: data,
				Time: time.Now(),
			}

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

func crawlWebPage(Weburl string) ScrapedData {
	// Create a new Colly collector.
	c := colly.NewCollector(
		colly.MaxDepth(3),
		colly.Async(true),
	)
	// c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2})

	var data ScrapedData
	// URL to start scraping
	startURL := Weburl

	// visited := make(map[string]struct{})
	var mutex sync.Mutex

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		// Extract and print links
		link := e.Request.AbsoluteURL(e.Attr("href"))
		fmt.Printf("Link: %s\n", link)

		if !visitedURLs[link] {
			mutex.Lock()
			data.Links = append(data.Links, link)
			visitedURLs[link] = true
			mutex.Unlock()
			e.Request.Visit(link)
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
	// Wait for the crawler to finish.
	c.Wait()

	return data
}
