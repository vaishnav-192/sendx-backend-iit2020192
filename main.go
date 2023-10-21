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

type request struct {
	url  string
	paid bool
}

type queue struct {
	mutex    sync.Mutex
	requests []request
}

func (q *queue) enqueue(r request) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if r.paid {
		q.requests = append([]request{r}, q.requests...)
	} else {
		q.requests = append(q.requests, r)
	}
}

func (q *queue) dequeue() request {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	r := q.requests[0]
	q.requests = q.requests[1:]

	return r
}

func (q *queue) isOpen() bool {
	return len(q.requests) > 0
}

func (q *queue) close() {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.requests = nil
}

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

	// Create a new waitgroup to track the completion of the goroutine.
	var wg sync.WaitGroup

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

		// Create a new queue to store the requests.
		q := queue{}
		wg.Add(1)

		// Add the request to the queue.
		q.enqueue(request{
			url:  url,
			paid: customerType == "Paid",
		})

		// Start a goroutine to process the requests in the queue.
		go func() {
			defer wg.Done()
			for {
				if q.isOpen() {
					req := q.dequeue()
					// Process the request.
					processRequest(w, req)
				} else {
					q.close()
					return
				}
			}
		}()

	} else {
		http.Error(w, "Web page not found", http.StatusNotFound)
	}

	// Wait for the goroutine to finish before returning the response to the user.
	wg.Wait()
}

func processRequest(w http.ResponseWriter, r request) {

	url := r.url

	mu.Lock()
	defer mu.Unlock()

	cache, present := Cachedata[url]
	if present {
		fmt.Printf("data found in cache. Serving from cache...\n")
		writeToUser(w, cache.data)
	} else {

		// Crawl the web page and scrape data and links
		data := crawlWebPage(url)

		Cachedata[url] = cachedata{
			url:  url,
			data: data,
			Time: time.Now(),
		}

		// Return the scraped data as JSON
		writeToUser(w, data)
	}

}

func writeToUser(w http.ResponseWriter, data ScrapedData) {
	jsonResponse, err := json.Marshal(data)
	if err != nil {
		http.Error(w, "Failed to marshal JSON", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")

	w.Write(jsonResponse)

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
		// colly.Async(true),
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
