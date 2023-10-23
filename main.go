package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gocolly/colly/v2"
)

type ScrapedData struct {
	Links []string `json:"links"`
}

var mu sync.Mutex

var visitedURLs = make(map[string]bool)

type request struct {
	url string
}

type queue struct {
	mutex    sync.Mutex
	requests []request
}

func (q *queue) enqueue(r request) {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	q.requests = append(q.requests, r)
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

var redisClient *redis.Client

func initRedisClient() {
	redisClient = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379", // Your Redis server address
		Password: "",               // No password by default
		DB:       0,                // Default DB
	})
}

func setDataWithTTL(key string, value ScrapedData) error {
	ctx := context.Background()

	// Serialize the struct into a JSON string
	jsonData, err := json.Marshal(value)
	if err != nil {
		return err
	}

	// Store the JSON data in Redis
	err = redisClient.Set(ctx, key, string(jsonData), 1*time.Hour).Err()
	if err != nil {
		return err
	}

	return nil
}

func getDataFromRedis(key string) (ScrapedData, error) {
	ctx := context.Background()
	// Retrieve the JSON data from Redis
	jsonData, err := redisClient.Get(ctx, key).Result()
	if err != nil {
		return ScrapedData{}, err
	}
	// Deserialize the JSON string into the ScrapedData struct
	var data ScrapedData
	if err := json.Unmarshal([]byte(jsonData), &data); err != nil {
		return ScrapedData{}, err
	}

	return data, nil
}

func main() {

	http.HandleFunc("/crawl", crawlhandler)
	http.Handle("/", http.FileServer(http.Dir(".")))
	initRedisClient()

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
	var wg1 sync.WaitGroup
	var wg2 sync.WaitGroup

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
		Paidq := queue{}
		Freeq := queue{}
		wg1.Add(5)
		wg2.Add(2)

		// Add the request to the queue.
		if customerType == "Paid" {
			Paidq.enqueue(request{
				url: url,
			})
		} else {
			Freeq.enqueue(request{
				url: url,
			})
		}

		for i := 0; i < 5; i++ {
			go func() {
				defer wg1.Done()
				for {
					if Paidq.isOpen() {
						req := Paidq.dequeue()
						processRequest(w, req)
					} else {
						Paidq.close()
						return
					}
				}
			}()
		}
		for i := 0; i < 2; i++ {
			go func() {
				defer wg2.Done()
				for {
					if Freeq.isOpen() {
						req := Freeq.dequeue()
						processRequest(w, req)
					} else {
						Freeq.close()
						return
					}
				}
			}()
		}

	} else {
		http.Error(w, "Web page not found", http.StatusNotFound)
	}

	// Wait for the goroutine to finish before returning the response to the user.
	wg1.Wait()
	wg2.Wait()
}

func processRequest(w http.ResponseWriter, r request) {

	url := r.url

	mu.Lock()

	// cache, present := Cachedata[url]

	// Get data
	cacheData, err := getDataFromRedis(url)
	if err != nil {
		fmt.Printf("data not found in cache\n")

		// Crawl the web page and scrape data and links
		data := crawlWebPage(url)

		// Set data with a TTL of 60 seconds
		err := setDataWithTTL(url, data)
		if err != nil {
			fmt.Println("Error setting data:", err)
		} else {
			fmt.Println("Data set successfully with TTL 1 hr.")
		}

		// Return the scraped data as JSON
		writeToUser(w, data)

	} else {
		fmt.Printf("Cache hit. Serving from cache...\n")
		writeToUser(w, cacheData)
	}

	defer mu.Unlock()
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
