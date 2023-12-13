package main

import (
	"encoding/csv"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net/http"
	"os"
	"strconv"
)
import "log"

func main() {
	url := "URL à inscrire ici"
	//var wg sync.WaitGroup
	type Radio struct {
		url     string
		title   string
		email   string
		genre   string
		country string
	}

	subpages := make([]Radio, 0)
	// todo find pages number
	pagination := 67
	//create csv file
	file, err := os.Create("fr.csv")
	defer file.Close()
	if err != nil {
		log.Fatalln(err)
	}
	// loop on pagination
	for i := 0; i < pagination; i++ {
		response, err := http.Get(url + "/fr/?p=" + strconv.FormatInt(int64(i), 10))

		if err != nil {
			log.Fatalln(err)
		}

		if response.StatusCode == 200 {

			document, err := goquery.NewDocumentFromReader(response.Body)

			if err != nil {
				log.Fatalln(err)
			}

			document.Find("ul#stations > li").Each(func(index int, selector *goquery.Selection) {
				title := selector.Find(".station__title__name").Text()
				radioSlug, _ := selector.Find("figure > a").Attr("href")

				subpages = append(subpages, Radio{
					url:   url + radioSlug,
					title: title,
				})
			})
			w := csv.NewWriter(file)
			defer w.Flush()
			for j, page := range subpages {
				res, err := http.Get(page.url)
				if err != nil {
					log.Fatalln(err)
				}

				doc, err := goquery.NewDocumentFromReader(res.Body)
				if err != nil {
					log.Fatalln(err)
				}
				subpages[j].country = doc.Find("ul.breadcrumbs li").First().Find("a span").Text()
				genre := ""
				doc.Find("ul.station__tags li a").Each(func(i int, selection *goquery.Selection) {
					genre += selection.Text() + ","
				})
				if len(genre) > 0 {
					subpages[j].genre = genre[0 : len(genre)-1] //remove trailing comma
				} else {
					subpages[j].genre = genre
				}
				subpages[j].email = doc.Find("a[itemprop=email]").First().Text()
				// write in file

				row := []string{subpages[j].country, subpages[j].title, subpages[j].email, subpages[j].genre}
				if err := w.Write(row); err != nil {
					log.Fatalln(err)
				}
			}

		} else {
			log.Fatalln("no scrape")
		}
	}
	fmt.Println("done", len(subpages))

}
