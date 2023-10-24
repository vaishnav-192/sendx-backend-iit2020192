# sendx-backend-iit2020192
Welcome to my Go web application README!<br>This document will provide you with essential information about the project, its features, how to set it up, and how to use it.
I have implemented all 3 parts of the assignment:
<ul>
  <li>Required</li>
  <li>Good to have</li>
<li>Great to have</li>
</ul>

## Tech used:

<ul>
	<li>Go</li>
	<li>HTML</li>
	<li>CSS</li>
	<li>Redis</li>
	<li>Postman (for testing)</li>
</ul>

## Features used:

<ul>
	<li>Checks the existance of webURL giving by the user (based on status code returned). If also retires to check 3 time (can be changes according to user's choice)</li>
	<li>It maintains 2 queues = PaidCustomers, FreeCustomers. I have assigned 5 crawler workers for PaidCustomers and 2 crawler workers for FreeCustomers. The crawler workers can crawl multiple pages concurrently.</li>
	<li>Before Processing a request, we first check if the data is present in cache or not. If present, we simple fetch data from cache and return. This significantly improves application performance and response time.</li>
	<li>This application crawls the webpage using gocolly, at the rate depth=2. It maintains a visited array to store those urls which are already visited (to reduce redundance of the application to crawl same weblinks again and again)</li>
	<li>I have commented out</li>

	// c.Limit(&colly.LimitRule{DomainGlob: "*", Parallelism: 2})
This can also improve perfomance significantly by restricting ourselves to same domain and parellel crawlling.
<li>Implemented APIs to controll maximum number of crawler workers that can crawl at a time and maximum number of requets/pages that can be crawled in a span of 1 hour</li>
</ul>

## Steps for Installation:
<ul>
	<li>Start by cloning the repo:</li>
	
	git clone https://github.com/vaishnav-192/sendx-backend-iit2020192
	cd sendx-backend-iit2020192

<li>Get required Dependencies:</li>

	go get github.com/go-redis/redis/v8
 	github.com/gocolly/colly/v2
</ul>

  ## Steps to run the Application:

  <ul>

 <li>Run Redis-server from ~/Redis/redis-server. Do not close the Redis-server, keep it running in background</li>
 <li>Run main.go</li>

 	go  run main.go

![runningApp](images/runningApp.png)

<li>open chrome (or any other web browser). Enter URL: http://localhost:8080</li>

	http://localhost:8080

![UI](images/webUI.png)

<li>Enter your desired website to start crawling and clink on submit button.</li>

![testWebsite](images/testWebsite.png)

<li>We can see application will start crawling on the webpage</li>

![crawling](images/crawling.png)

<li>After crawling is finished, user will get an output JSON file, containing all the links application crawled so far (until depth=2, can be changed according to user needs in code)</li>

![outputJSON](images/outputDataJSON.png)

<li>After Crawling we cache the data fetched in Redis with 1hr TTL(time to live), as show in the following image</li>

![dataCached](images/dataCached.png)

<li>Whenever that website is called again to crawl(within a time gap of 1hr), we return the data from cache. Website is not crawled, as shown below:</li>

![cacheHit](images/cacheHit.png)

<li>We can use Postman application also.<br>Setup for Postman application:</li>

![postman](images/pstQuery.png)

<li>We can use API calls from postman to run the application</li>

![postmanRunning](images/pstQueryRunning.png)

<li>Pay attention to the time taken to fetch data for a website for first time (image is attached below). There will be cache miss. As a result, the time taken to fetch the data is huge.</li>

![cacheMiss](images/cacheMissTime.png)

<li>If user calls the same website again within 1hr of last time call, we can see this time cache was hit!! <br>Pay attention to the time taken to fetch the data. It is significantly faster than before!!</li>

![cacheHit](images/cacheHitTime.png)

<li>We can also give inputs using curl (or Invoke-WebRequest) by following steps:</li>

<ul>
	<li>create a JSON type variable using '$' sign example : </li>

 		$parameter = @{
	 	url = 'https://example.com'
		customerType = 'Paid'
		}

![parameters](images/parameters.png)

if 'customerType' field is not given, bu default it takes 'Free' customer.

<li>Use the following command to invoke the web application as shown in image</li>

![comand](images/invokeWebrequest.png)
</ul>
 <li>We can update max no. of pages/request and crawler workers and even see no. of active crawler workers and total no. of requests served in that hour through these following APIs:</li>

 		http://localhost:8080/pages?pages=(int)
 		http://localhost:8080/workers?workers=(int)
 		http://localhost:8080/getworkers   		
 		http://localhost:8080/getpages   		
 		http://localhost:8080/getCurrPages   		
 		http://localhost:8080/getCurrWorkers

   Example: (output is printed in console)
   ![setPages](images/UpdatingCrawlLimit.png)
   ![getPages](images/getPages.png)
   <li>
	   If we spam requests, two things may happen:
	   <ul>
		   <li>out of workers due to set max no. of workers (that can work at a instance)</li>
Example: spamming request

![limitPage1](images/limitPage1.png)	 	

Out of requests
![limitPage2](images/limitPage2.png)	 	
<li>out of requests due to hourly set limit (refreshes every hour)</li>

Example: spamming workers

![limitWorker](images/limitWorker.png)
		</ul>
   </li>
</ul>

### Major takeawys from this project:
<ul>
	<li>Understanding working/exploring Golang --Added Experience</li>
	<li>Hands on experience of building a scalable application</li>
	<li>error handeling</li>
</ul>

## Demo Video Link:

[YouTubeLink](https://www.youtube.com/watch?v=JZYBC0TkDKI)
<br>
If it is not working try:
<br>
[ClickHere](https://clipchamp.com/watch/xaxjQRZ5BXq)

<table>
  <tr>
    <td align="center"><a href="https://github.com/vaishnav-192"><img src="https://avatars.githubusercontent.com/u/76587671?v=4" width="150px;" alt=""/><br /><sub><b>Vaishnav Kamarti</b></sub></a><br />IIT2020192</td>
 </tr>
</table>

