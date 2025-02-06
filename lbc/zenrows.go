package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
)

type SearchResults struct {
	Links []string `json:"links"`
}

// récupère les liens d'annonces depuis la recherche lbc
func search_ad_links() ([]string, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.zenrows.com/v1/?apikey=523cc0567faf890d60602f97181b9128a134c8ea&url=https%3A%2F%2Fwww.leboncoin.fr%2Frecherche%3Fcategory%3D10%26locations%3Dr_23%26real_estate_type%3D1%2C2&js_render=true&wait_for=h2.text-subhead-expanded&premium_proxy=true&proxy_country=fr&css_extractor=%257B%2522links%2522%253A%2522a%255Bdata-qa-id%253D%27aditem_container%27%255D%2520%2540href%2522%257D", nil)
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}

	var response SearchResults
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	return response.Links, nil
}

func get_ad_data(path string) (AdResult, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://api.zenrows.com/v1/?apikey=523cc0567faf890d60602f97181b9128a134c8ea&url=https%3A%2F%2Fwww.leboncoin.fr"+url.QueryEscape(path)+"&css_extractor=%257B%2522images%2522%253A%2522section%255Bdata-qa-id%253D%27adview_spotlight_container%27%255D%2520img%2520%2540src%2522%252C%2522price%2522%253A%2522div%255Bdata-qa-id%253D%27adview_price%27%255D%2520p.text-headline-2%2522%252C%2522desc%2522%253A%2522div%255Bdata-qa-id%253D%27adview_spotlight_description_container%27%255D%2520p%2520%253E%2520span%2522%257D", nil)
	if err != nil {
		log.Fatalln(err)
		return AdResult{}, err
	}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatalln(err)
		return AdResult{}, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Fatalln(err)
	}
	res := string(body)
	log.Println(res)

	var result AdResult
	error := json.Unmarshal([]byte(res), &result)
	if error != nil {
		fmt.Println("Error:", err)
		return AdResult{}, err
	}
	return result, nil
}

type AdResult struct {
	Images []string `json:"images"`
	Price  []string `json:"price"`
	Desc   []string `json:"desc"`
}
type AdData struct {
	Desc   string   `json:"desc"`
	Url    string   `json:"url"`
	Images []string `json:"images"`
	Price  string   `json:"price"`
}

// Functions for managing the existisng ads data inMemory
type AdMap struct {
	ads map[string]AdData
	mu  sync.RWMutex
}

func NewAdMap() *AdMap {
	return &AdMap{
		ads: make(map[string]AdData),
	}
}

func (am *AdMap) Load(filename string) error {
	file, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, start with empty map
		}
		return err
	}

	var ads []AdData
	err = json.Unmarshal(file, &ads)
	if err != nil {
		return err
	}

	am.mu.Lock()
	defer am.mu.Unlock()
	for _, ad := range ads {
		am.ads[ad.Url] = ad
	}
	return nil
}

func (am *AdMap) Exists(url string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()
	_, exists := am.ads[url]
	return exists
}

func (am *AdMap) Add(ad AdData) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.ads[ad.Url] = ad
}

func (am *AdMap) SaveToFile(filename string) error {
	am.mu.RLock()
	defer am.mu.RUnlock()

	ads := make([]AdData, 0, len(am.ads))
	for _, ad := range am.ads {
		ads = append(ads, ad)
	}

	data, err := json.MarshalIndent(ads, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0644)
}

// func readJSONAndProcessLinks(filename string) error {
// 	// Read the file
// 	fileContent, err := os.ReadFile(filename)
// 	if err != nil {
// 		return fmt.Errorf("error reading file: %w", err)
// 	}

// 	// Parse JSON
// 	var data SearchResults
// 	err = json.Unmarshal(fileContent, &data)
// 	if err != nil {
// 		return fmt.Errorf("error unmarshaling JSON: %w", err)
// 	}

// 	// Loop through links
// 	// get_adList_data(data)
// 	// log.Println(ad_list)

// 	return nil
// }

// get unique images urls from slice of images
func getUniqueImages(images []string) []string {
	uniqueImages := make(map[string]bool)
	for _, img := range images {
		uniqueImages[img] = true
	}

	result := make([]string, 0, len(uniqueImages))
	for img := range uniqueImages {
		result = append(result, img)
	}
	return result
}

func get_adList_data_concurrent(adMap *AdMap, links []string, concurrency int) []AdData {
	var ad_list []AdData
	var mu sync.Mutex
	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, link := range links {
		wg.Add(1)
		sem <- struct{}{}
		go func(link string) {
			defer wg.Done()
			defer func() { <-sem }()
			if adMap.Exists(link) {
				log.Printf("Ad already exists, skipping: %s", link)
				return
			}
			ad_data, err := get_ad_data(link)
			if err != nil || len(ad_data.Images) == 0 {
				return
			}

			uniqueImages := getUniqueImages(ad_data.Images)
			price_value := ad_data.Price[0][:len(ad_data.Price[0])-1]
			new_ad := AdData{Url: link, Images: uniqueImages, Price: price_value, Desc: strings.Join(ad_data.Desc, "")}

			mu.Lock()
			ad_list = append(ad_list, new_ad)
			mu.Unlock()
			adMap.Add(new_ad)
		}(link)
	}

	wg.Wait()
	return ad_list
}

func appendAdListToJSON(newAd AdData) error {
	filename := "ad_data01.json"
	var existingAdList []AdData

	// Read existing file if it exists
	fileContent, err := os.ReadFile(filename)
	if err == nil {
		// File exists, unmarshal its content
		err = json.Unmarshal(fileContent, &existingAdList)
		if err != nil {
			return fmt.Errorf("error unmarshaling existing JSON: %w", err)
		}
	} else if !os.IsNotExist(err) {
		// Error other than file not existing
		return fmt.Errorf("error reading file: %w", err)
	}

	// Append new data to existing data
	existingAdList = append(existingAdList, newAd)

	// Marshal the combined data
	jsonData, err := json.MarshalIndent(existingAdList, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling JSON: %w", err)
	}

	// Write the updated data back to the file
	err = os.WriteFile(filename, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("error writing file: %w", err)
	}

	fmt.Printf("Successfully appended new item to %s\n", filename)
	return nil
}

func main() {
	filename := "ad_data01.json"
	adMap := NewAdMap()
	err := adMap.Load(filename)
	if err != nil {
		log.Fatalf("Error loading existing ads: %v", err)
	}

	links, err := search_ad_links()
	if err != nil {
		log.Fatalf("Error fetching links: %v", err)
	}
	concurrency := 5 // You can adjust this value based on your needs

	newAds := get_adList_data_concurrent(adMap, links, concurrency)

	log.Printf("Processed %d new ads", len(newAds))

	err = adMap.SaveToFile(filename)
	if err != nil {
		log.Fatalf("Error saving ads to file: %v", err)
	}

}
