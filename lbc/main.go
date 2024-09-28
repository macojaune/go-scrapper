package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"github.com/gocolly/colly/v2"
)

type Offer struct {
	price    string
	title    string
	rooms    int
	pictures []string
	url      string
}

func main() {

	// Instantiate default collector
	c := colly.NewCollector(
		// Visit only domains:
		colly.AllowedDomains("leboncoin.fr", "www.leboncoin.fr"),

		// Cache responses to prevent multiple download of pages
		// even if the collector is restarted
		colly.CacheDir("./lbc_cache"),
		colly.AllowURLRevisit(),
	)

	// Create another collector to scrape course details
	// detailCollector := c.Clone()

	offers := make([]Offer, 0, 200)

	// On every <a> element which has "href" attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		// If attribute class is this long string return from callback
		// As this a is irrelevant
		log.Println("link", e.Attr("href"))
		if !strings.HasPrefix(e.Attr("href"), "/ad/locations") {
			return
		}
		link := e.Attr("href")
		// If link start with browse or includes either signup or login return from callback
		//  if !strings.HasPrefix(link, "/browse") || strings.Index(link, "=signup") > -1 || strings.Index(link, "=login") > -1 {
		// 	return
		// }
		// start scaping the page under the link found
		e.Request.Visit(link)
	})

	// Before making a request print "Visiting ..."
	c.OnRequest(func(r *colly.Request) {
		log.Println("visiting", r.URL.String())
	})

	// Set error handler
	c.OnError(func(r *colly.Response, err error) {
		fmt.Println("Request URL:", r.Request.URL, "\nError:", err)
	})

	// // On every <a> element with collection-product-card class call callback
	c.OnHTML(`main`, func(e *colly.HTMLElement) {
		title := e.ChildText("h1")
		log.Println("title found", title)

		if title == "" {
			log.Println("No title found", e.Request.URL)
		}
		rooms, err := strconv.Atoi(e.ChildText("div[data-qa-id='criteria_item_rooms'] > span"))
		if err != nil {
			log.Println("No Rooms found")
			rooms = 0
		}

		offer := Offer{
			title: title,
			url:   e.Request.URL.String(),
			price: e.ChildText("div[data-qa-id='adview_price']"),
			rooms: rooms,
		}
		// Iterate over div components and add details to course
		e.ForEach("button picture img[alt]", func(_ int, el *colly.HTMLElement) {
			if strings.Index(e.Attr("alt"), "Image") != -1 {
				offer.pictures = append(offer.pictures, e.Attr("src"))
			}
		})
		log.Printf("%+v\n", offer)
		offers = append(offers, offer)

	})

	// Start scraping on lbc
	// r_23 is the code for Guadeloupe
	c.Visit("https://www.leboncoin.fr/locations/offres?locations=r_23")
}
