package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

type RentalProperty struct {
	Platform string `json:"platform"`
	Title    string `json:"title"`
	Price    string `json:"price"`
	Location string `json:"location"`
	URL      string `json:"url"`
	Owner    string `json:"owner"`
	Address  string `json:"address"`
	Type     string `json:"type"`
	Capacity string `json:"capacity"`
	City string `json:"city"`
}

func createBrowser() (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", false),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent(`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36`),
		chromedp.WindowSize(1920, 1080),
		// chromedp.ProxyServer("https://superproxy.zenrows.com:1338"),
	)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), opts...)

	ctx, cancel := chromedp.NewContext(allocCtx)

	// // Add timeout
	// ctx, cancel = context.WithTimeout(ctx, 30*time.Second)

	return ctx, func() {
		cancel()
		allocCancel()
	}
}

func scrapeAirbnb(ctx context.Context, location string, filename string) error {
	url := fmt.Sprintf("https://www.airbnb.com/s/%s/homes", location)

	// Navigate to the first page
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		return err
	}

	var urls []string

	// Loop through pages (limit to 5 pages for example)
	for page := 0; page < 15; page++ {
		log.Println("Processing page: ", page)
		var page_urls []string

		err := chromedp.Run(ctx,
			chromedp.Sleep(3*time.Second), // Wait for page to load
			chromedp.Evaluate(`Array.from(document.querySelectorAll('meta[itemprop="url"]')).map(el => el.content)`, &page_urls),
		)
		if err != nil {
			log.Printf("Error extracting URLs on page %d: %v", page+1, err)
			break
		}
		urls = append(urls, page_urls...)

		// Try to go to next page
		var hasNextPage bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`!!document.querySelector('a[aria-label="Suivant"]:not([disabled])')`, &hasNextPage),
		)
		if err != nil || !hasNextPage {
			log.Println("No more pages or error checking next page:", err)
			break
		}

		// Click next page button
		err = chromedp.Run(ctx,
			chromedp.Click("a[aria-label*='Suivant']", chromedp.ByQuery),
		)

		if err != nil {
			log.Println("Error navigating to next page: ", err)
			break
		}

	}
	log.Println("airbnb URLS: ", len(urls))
	// Process URLs from current page
	// Process URLs concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 4) // Limit concurrent requests

	for index, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create new browser context for each goroutine
			newCtx, cancel := createBrowser()
			defer cancel()

			property := RentalProperty{
				Platform: "Airbnb",
				URL:      "https://" + url,
				Location: location,
			}

			log.Println("Processing URL: ", index)

			err := chromedp.Run(newCtx,
				chromedp.Navigate(property.URL),
				chromedp.Sleep(4*time.Second),
				chromedp.TextContent("h1", &property.Title, chromedp.ByQuery),
				chromedp.TextContent("div[data-testid='book-it-default'] div[aria-hidden='true'] span", &property.Price, chromedp.ByQuery),
				chromedp.AttributeValue("a[href*='/users/show'][aria-label*='hôte']", "href", &property.Owner, nil, chromedp.ByQuery),
			)

			if err != nil {
				log.Printf("Error processing URL %s: %v", property.URL, err)
				return
			}

			property.Owner = "https://www.airbnb.fr" + property.Owner

			mu.Lock()
			// Write single property to file
			existingProperties, err := loadExistingProperties(filename)
			if err != nil {
				log.Printf("Error loading properties: %v", err)
				mu.Unlock()
				return
			}

			// Check for duplicates
			exists := false
			for _, ep := range existingProperties {
				if ep.URL == property.URL {
					exists = true
					break
				}
			}

			if !exists {
				existingProperties = append(existingProperties, property)
				data, err := json.MarshalIndent(existingProperties, "", "  ")
				if err != nil {
					log.Printf("Error marshaling data: %v", err)
					mu.Unlock()
					return
				}

				if err := os.WriteFile(filename, data, 0644); err != nil {
					log.Printf("Error saving to file: %v", err)
				}
			}
			mu.Unlock()
		}(url)
	}

	wg.Wait()
	return nil
}

func scrapeBooking(ctx context.Context, location string, filename string) error {
	url := fmt.Sprintf("https://www.booking.com/searchresults.html?ss=%s", location)
	var mu sync.Mutex
	var wg sync.WaitGroup
	var urls []string

	// Use chromedp to get property URLs
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(2*time.Second),
		chromedp.ScrollIntoView("a.footer-top-button", chromedp.ByQuery),
		chromedp.Sleep(300*time.Millisecond),
		chromedp.ScrollIntoView("a.footer-top-button", chromedp.ByQuery),
		chromedp.Evaluate(`Array.from(document.querySelectorAll('a[data-testid="title-link"]')).map(el => el.href)`, &urls),
	)

	if err != nil {
		log.Printf("Error extracting URLs on page: %d", err)
		return err
	}

	log.Println("booking URLS: ", len(urls))
	// Create a semaphore to limit concurrent browser instances
	semaphore := make(chan struct{}, 5)

	for index, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create new browser context for each goroutine
			newCtx, cancel := createBrowser()
			defer cancel()

			property := RentalProperty{
				Platform: "Booking",
				URL:      url,
				Location: location,
			}

			log.Println("Processing URL: ", index)

			// First check if we can process the URL
			err := chromedp.Run(newCtx,
				chromedp.Navigate(property.URL),
				chromedp.Sleep(4*time.Second),
				chromedp.TextContent("#hp_hotel_name h2", &property.Title, chromedp.ByQuery),
				chromedp.TextContent("div[data-testid='PropertyHeaderAddressDesktop-wrapper'] span > div:first-child", &property.Address, chromedp.ByQuery),
				chromedp.TextContent("table td div a span", &property.Type, chromedp.ByQuery),
				chromedp.TextContent("div[data-testid='host-profile'] h2", &property.Owner, chromedp.ByQuery),
			)

			if err != nil {
				log.Printf("Error processing URL %s: %v", property.URL, err)
				return
			}

			mu.Lock()
			existingProperties, err := loadExistingProperties(filename)
			mu.Unlock()

			if err != nil {
				log.Printf("Error loading properties: %v", err)
				return
			}

			// Check for duplicates
			exists := false
			for _, ep := range existingProperties {
				if ep.URL == property.URL {
					exists = true
					break
				}
			}

			if !exists {
				existingProperties = append(existingProperties, property)
				data, err := json.MarshalIndent(existingProperties, "", "  ")
				if err != nil {
					log.Printf("Error marshaling data: %v", err)
					return
				}

				mu.Lock()
				err = os.WriteFile(filename, data, 0644)
				mu.Unlock()

				if err != nil {
					log.Printf("Error saving to file: %v", err)
					return
				}
			}
		}(url)
	}

	wg.Wait()
	return nil
}

func scrapeAbritel(ctx context.Context, location string, filename string) error {
	// url := fmt.Sprintf("https://www.abritel.fr/search/destination=%s", location)
	url := "https://www.tripadvisor.fr/VacationRentals-g2108743-Reviews-Lamentin_Basse_Terre_Island_Guadeloupe-Vacation_Rentals.html"

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		go func() {
			switch ev := ev.(type) {
			case *fetch.EventAuthRequired:
				c := chromedp.FromContext(ctx)
				execCtx := cdp.WithExecutor(ctx, c.Target)

				resp := &fetch.AuthChallengeResponse{
					Response: fetch.AuthChallengeResponseResponseProvideCredentials,
					Username: "GLgynXvGK7FG",
					Password: "c3uyspTDhfAt",
				}

				err := fetch.ContinueWithAuth(ev.RequestID, resp).Do(execCtx)
				if err != nil {
					log.Print(err)
				}

			case *fetch.EventRequestPaused:
				c := chromedp.FromContext(ctx)
				execCtx := cdp.WithExecutor(ctx, c.Target)
				err := fetch.ContinueRequest(ev.RequestID).Do(execCtx)
				if err != nil {
					log.Print(err)
				}
			}
		}()
	})

	err := chromedp.Run(ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			network.SetExtraHTTPHeaders(map[string]interface{}{
				"Accept-Language": "fr-Fr,fr;q=0.9",
				"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
				"DNT":             "1",
				"Connection":      "keep-alive",
			})
			return nil
		}),
		fetch.Enable().WithHandleAuthRequests(true),
		chromedp.Navigate(url),
	)
	if err != nil {
		log.Printf("Error visiting URL: %s, %s", url, err)
	}

	var urls []string
	// Loop through pages (limit to 5 pages for example)
	for page := 0; page < 5; page++ {
		log.Println("Processing page: ", page)
		var page_urls []string

		err = chromedp.Run(ctx,
			chromedp.Sleep(3*time.Second),
			chromedp.Evaluate(`Array.from(document.querySelectorAll('h2 > a')).map(el => el.href)`, &page_urls),
		)
		if err != nil {
			log.Printf("Error extracting URLs on page: %d, err: %s", page, err)
			break
		}
		log.Println("URLS: ", len(page_urls))

		urls = append(urls, page_urls...)

		// Try to go to next page
		var hasNextPage bool
		err = chromedp.Run(ctx,
			chromedp.Evaluate(`!!document.querySelector("a[data-smoke-attr='pagination-next-arrow']")`, &hasNextPage),
		)
		if err != nil || !hasNextPage {
			log.Println("No more pages or error checking next page:", err)
			break
		}

		// Click next page button
		err = chromedp.Run(ctx,
			chromedp.Click("a[data-smoke-attr='pagination-next-arrow']", chromedp.ByQuery),
		)

		if err != nil {
			log.Println("Error navigating to next page: ", err)
			break
		}
	}
	log.Println("tripadvisor URLS: ", len(urls))
	// Process URLs from current page
	// Process URLs concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	semaphore := make(chan struct{}, 1) // Limit concurrent requests

	for index, url := range urls {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()

			// Acquire semaphore
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Create new browser context for each goroutine
			newCtx, cancel := createBrowser()
			defer cancel()

			property := RentalProperty{
				Platform: "TripAdvisor",
				URL:      url,
				Location: "Basse-Terre",
			}

			log.Println("Processing URL: ", index)
			chromedp.ListenTarget(newCtx, func(ev interface{}) {
				go func() {
					switch ev := ev.(type) {
					case *fetch.EventAuthRequired:
						c := chromedp.FromContext(newCtx)
						execCtx := cdp.WithExecutor(newCtx, c.Target)

						resp := &fetch.AuthChallengeResponse{
							Response: fetch.AuthChallengeResponseResponseProvideCredentials,
							Username: "GLgynXvGK7FG",
							Password: "c3uyspTDhfAt",
						}

						err := fetch.ContinueWithAuth(ev.RequestID, resp).Do(execCtx)
						if err != nil {
							log.Print(err)
						}

					case *fetch.EventRequestPaused:
						c := chromedp.FromContext(newCtx)
						execCtx := cdp.WithExecutor(newCtx, c.Target)
						err := fetch.ContinueRequest(ev.RequestID).Do(execCtx)
						if err != nil {
							log.Print(err, ev.Request.URL)
						}
					}
				}()
			})
			err := chromedp.Run(newCtx,
				chromedp.ActionFunc(func(ctx context.Context) error {
					network.SetExtraHTTPHeaders(map[string]interface{}{
						"Accept-Language": "fr-Fr,fr;q=0.9",
						"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8",
						"DNT":             "1",
						"Connection":      "keep-alive",
					})
					return nil
				}),
				fetch.Enable().WithHandleAuthRequests(true),
				chromedp.Navigate(property.URL),
				chromedp.Sleep(4*time.Second),
				chromedp.TextContent("h1", &property.Title, chromedp.ByQuery),
				chromedp.TextContent("div[data-automation='nightlyPrice']", &property.Price, chromedp.ByQuery),
				chromedp.TextContent("#vr-detail-page-overview div div div svg + div", &property.Type, chromedp.ByQuery),
				chromedp.TextContent("#vr-detail-page-overview div div div:nth-child(3) svg + div", &property.Capacity, chromedp.ByQuery),
			)

			if err != nil {
				log.Printf("Error processing URL %s: %v", property.URL, err)
				return
			}

			mu.Lock()
			// Write single property to file
			existingProperties, err := loadExistingProperties(filename)
			if err != nil {
				log.Printf("Error loading properties: %v", err)
				mu.Unlock()
				return
			}

			// Check for duplicates
			exists := false
			for _, ep := range existingProperties {
				if ep.URL == property.URL {
					exists = true
					break
				}
			}

			if !exists {
				existingProperties = append(existingProperties, property)
				data, err := json.MarshalIndent(existingProperties, "", "  ")
				if err != nil {
					log.Printf("Error marshaling data: %v", err)
					mu.Unlock()
					return
				}

				if err := os.WriteFile(filename, data, 0644); err != nil {
					log.Printf("Error saving to file: %v", err)
				}
			}
			mu.Unlock()
		}(url)
	}

	wg.Wait()
	return nil
}

func scrapeAllPlatforms(location string, filename string) {

	platforms := []struct {
		name    string
		scraper func(context.Context, string, string) error
	}{
		{"Airbnb", scrapeAirbnb},
		{"Booking", scrapeBooking},
		// {"Abritel", scrapeAbritel},
	}

	var wg sync.WaitGroup
	for _, platform := range platforms {
		wg.Add(1)
		go func(name string, scraper func(context.Context, string, string) error) {
			defer wg.Done()

			newCtx, cancel := createBrowser()
			defer cancel()

			if err := scraper(newCtx, location, filename); err != nil {
				log.Printf("Error scraping %s: %v", name, err)
			}
		}(platform.name, platform.scraper)
	}

	wg.Wait()
}

func loadExistingProperties(filename string) ([]RentalProperty, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return []RentalProperty{}, nil
		}
		return nil, err
	}

	var properties []RentalProperty
	if err := json.Unmarshal(data, &properties); err != nil {
		return nil, err
	}
	return properties, nil
}

func main() {
	locations := []string{
		// "Lamentin--Basse~Terre--Guadeloupe",
		"Sainte~Rose--Basse~Terre--Guadeloupe",
		"Deshaies--Basse~Terre--Guadeloupe",
		"Petit~Bourg--Basse~Terre--Guadeloupe",
		"Goyave--Basse~Terre--Guadeloupe",
	}

	for _, location := range locations {
		filename := fmt.Sprintf("vacation_rentals_%s.json",
			strings.ReplaceAll(location, "-", "_"))
		scrapeAllPlatforms(location, filename)
	}
}
