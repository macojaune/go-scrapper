package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/gocolly/colly/v2"
)

type Asso struct {
	Adress string
	Title  string
	Tel    string
	URL    string
}

func main() {
	fName := "assos.json"
	file, err := os.Create(fName)
	if err != nil {
		log.Fatalf("Cannot create file %q: %s\n", fName, err)
		return
	}
	defer file.Close()

	// Instantiate default collector
	c := colly.NewCollector(
		// Visit only domains: coursera.org, www.coursera.org
		colly.AllowedDomains("net1901.org", "www.net1901.org"),

		// Cache responses to prevent multiple download of pages
		// even if the collector is restarted
		colly.CacheDir("./scrap_cache"),
	)

	// Create another collector to scrape course details
	detailCollector := c.Clone()

	assos := make([]Asso, 0, 200)

	// On every <a> element which has "href" attribute call callback
	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		// If attribute class is this long string return from callback
		// As this a is irrelevant
		if !strings.HasPrefix(e.Attr("href"), "/annuaire-association/epci/CA-CAP-Excellence,19") {
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

	// On every <a> element with collection-product-card class call callback
	c.OnHTML(`a.list-group-item`, func(e *colly.HTMLElement) {
		// Activate detailCollector if the link contains "coursera.org/learn"
		assoURL := e.Request.AbsoluteURL(e.Attr("href"))
		if strings.Index(assoURL, "/association/") != -1 {
			detailCollector.Visit(assoURL)
		}
	})

	// Extract details of the course
	detailCollector.OnHTML(`span[itemtype="http://schema.org/LocalBusiness"] > div.container.theme-showcase`, func(e *colly.HTMLElement) {
		log.Println("Asso found", e.Request.URL)
		title := e.ChildText("span.h3Titre2")
		if title == "" {
			log.Println("No title found", e.Request.URL)
		}
		asso := Asso{
			Title:  title,
			URL:    e.Request.URL.String(),
			Adress: e.ChildText("span[itemprop=streetAddress]") + ", " + e.ChildText("span[itemprop=postalCode]") + ", " + e.ChildText("span[itemprop=addressLocality]"),
			Tel:    e.ChildText("span[itemprop=telephone]"),
		}
		log.Println(asso.Title, asso.Adress)
		// // Iterate over div components and add details to course
		// e.ForEach(".AboutCourse .ProductGlance > div", func(_ int, el *colly.HTMLElement) {
		// 	svgTitle := strings.Split(el.ChildText("div:nth-child(1) svg title"), " ")
		// 	lastWord := svgTitle[len(svgTitle)-1]
		// 	switch lastWord {
		// 	// svg Title: Available Languages
		// 	case "languages":
		// 		asso.Language = el.ChildText("div:nth-child(2) > div:nth-child(1)")
		// 	// svg Title: Mixed/Beginner/Intermediate/Advanced Level
		// 	case "Level":
		// 		asso.Level = el.ChildText("div:nth-child(2) > div:nth-child(1)")
		// 	// svg Title: Hours to complete
		// 	case "complete":
		// 		asso.Commitment = el.ChildText("div:nth-child(2) > div:nth-child(1)")
		// 	}
		// })
		assos = append(assos, asso)
	})

	// Start scraping on http://coursera.com/browse
	c.Visit("https://www.net1901.org/annuaire-association/epci/CA-CAP-Excellence,19.html")

	enc := json.NewEncoder(file)
	enc.SetIndent("", "  ")

	// Dump json to the standard output
	enc.Encode(assos)
}
