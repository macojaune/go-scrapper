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

	"github.com/chromedp/chromedp"
)

type RentalProperty struct {
	Platform string `json:"platform"`
	Title    string `json:"title"`
	Price    string `json:"price"`
	Location string `json:"location"`
	URL      string `json:"url"`
	Owner    string `json:"owner"`
}

func createBrowser() (context.Context, context.CancelFunc) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("no-sandbox", true),
		chromedp.UserAgent(`Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36`),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	ctx, cancel := chromedp.NewContext(allocCtx)
	return ctx, cancel
}

func scrapeAirbnb(ctx context.Context, location string) ([]RentalProperty, error) {
	var properties []RentalProperty
	url := fmt.Sprintf("https://www.airbnb.com/s/%s/homes", location)

	// Navigate to the first page
	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
	)
	if err != nil {
		return nil, err
	}

	var urls []string

	// Loop through pages (limit to 5 pages for example)
	for page := 0; page < 5; page++ {
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
	log.Println("URLS: ", len(urls))
	// Process URLs from current page
	for _, url := range urls {
		log.Println("Processing URL: ", url)
		property := RentalProperty{
			Platform: "Airbnb",
			URL:      "https://" + url,
			Location: location,
		}
		err := chromedp.Run(ctx,
			chromedp.Navigate(property.URL),
			chromedp.Sleep(4*time.Second), // Wait for navigation
			// chromedp.WaitVisible("h1", chromedp.ByQuery),
			chromedp.TextContent("h1", &property.Title, chromedp.ByQuery),
			chromedp.TextContent("h1", &property.Title, chromedp.ByQuery),
			chromedp.TextContent("div[data-testid='book-it-default'] div[aria-hidden='true'] span", &property.Price, chromedp.ByQuery),
			chromedp.AttributeValue("a[href*='/users/show'][aria-label*='hôte']", "href", &property.Owner, nil, chromedp.ByQuery),
		)
		property.Owner = "https://www.airbnb.fr" + property.Owner

		if err != nil {
			log.Printf("Error navigating to URL %s: %v", property.URL, err)
			continue
		}

		properties = append(properties, property)

	}
	return properties, nil
}

func scrapeBooking(ctx context.Context, location string) ([]RentalProperty, error) {
	var properties []RentalProperty
	url := fmt.Sprintf("https://www.booking.com/searchresults.html?ss=%s", location)

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
		// Add specific selectors and actions for Booking
	)

	return properties, err
}

func scrapeAbritel(ctx context.Context, location string) ([]RentalProperty, error) {
	var properties []RentalProperty
	url := fmt.Sprintf("https://www.abritel.fr/search/keywords:%s", location)

	err := chromedp.Run(ctx,
		chromedp.Navigate(url),
		chromedp.Sleep(5*time.Second),
		// Add specific selectors and actions for Abritel
	)

	return properties, err
}

func scrapeAllPlatforms(location string) []RentalProperty {
	var allProperties []RentalProperty
	var mu sync.Mutex
	var wg sync.WaitGroup

	_, cancel := createBrowser()
	defer cancel()

	platforms := []struct {
		name    string
		scraper func(context.Context, string) ([]RentalProperty, error)
	}{
		{"Airbnb", scrapeAirbnb},
		// {"Booking", scrapeBooking},
		// {"Abritel", scrapeAbritel},
	}

	for _, platform := range platforms {
		wg.Add(1)
		go func(name string, scraper func(context.Context, string) ([]RentalProperty, error)) {
			defer wg.Done()

			newCtx, cancel := createBrowser()
			defer cancel()

			properties, err := scraper(newCtx, location)
			if err != nil {
				log.Printf("Error scraping %s: %v", name, err)
				return
			}

			mu.Lock()
			allProperties = append(allProperties, properties...)
			mu.Unlock()
		}(platform.name, platform.scraper)
	}

	wg.Wait()
	return allProperties
}

func main() {
	locations := []string{
		"Lamentin--Basse~Terre--Guadeloupe",
		// "Sainte~Rose--Basse~Terre--Guadeloupe",
		// "Deshaies--Basse~Terre--Guadeloupe",
		// "Petit~Bourg--Basse~Terre--Guadeloupe",
		// "Goyave--Basse~Terre--Guadeloupe"
	}

	for _, location := range locations {
		properties := scrapeAllPlatforms(location)
		filename := fmt.Sprintf("vacation_rentals_%s.json",
			strings.ReplaceAll(location, "-", "_"))

		data, err := json.MarshalIndent(properties, "", "  ")
		if err != nil {
			log.Printf("Error marshaling data: %v", err)
			continue
		}

		if err := os.WriteFile(filename, data, 0644); err != nil {
			log.Printf("Error saving to file: %v", err)
		}
	}
}
