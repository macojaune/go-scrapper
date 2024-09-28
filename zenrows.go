package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
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
	req, err := http.NewRequest("GET", "https://api.zenrows.com/v1/?apikey=523cc0567faf890d60602f97181b9128a134c8ea&url=https%3A%2F%2Fwww.leboncoin.fr"+url.QueryEscape(path)+"&js_render=true&premium_proxy=true&css_extractor=%257B%2522images%2522%253A%2522section%255Bdata-qa-id%253D%27adview_spotlight_container%27%255D%2520img%2520%2540src%2522%252C%2522price%2522%253A%2522div%255Bdata-qa-id%253D%27adview_price%27%255D%2520p.text-headline-2%2522%257D", nil)
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
}
type AdData struct {
	Url    string   `json:"url"`
	Images []string `json:"images"`
	Price  string   `json:"price"`
}

func readJSONAndProcessLinks(filename string) error {
	// Read the file
	fileContent, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("error reading file: %w", err)
	}

	// Parse JSON
	var data SearchResults
	err = json.Unmarshal(fileContent, &data)
	if err != nil {
		return fmt.Errorf("error unmarshaling JSON: %w", err)
	}

	// Loop through links
	ad_list := get_adList_data(data)
	log.Println(ad_list)

	return nil
}

func get_adList_data(data SearchResults) []AdData {
	var ad_list []AdData

	for i, link := range data.Links {
		fmt.Printf("Link %d: %s\n", i+1, link)
		ad_data, err := get_ad_data(link)
		if err != nil || len(ad_data.Images) == 0 {
			continue
		}
		new_ad := AdData{Url: link, Images: ad_data.Images, Price: ad_data.Price[0]}

		// Write the ad list to a file
		err = appendAdListToJSON(new_ad)
		if err != nil {
			fmt.Println("error appending ad  to JSON list:", err)
		}
		ad_list = append(ad_list, new_ad)
	}
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
	//test with file
	readJSONAndProcessLinks("lbc.json")

	//fetch ad links
	// 	links, err := search_ad_links()
	// if err != nil {
	//     log.Fatalf("Error fetching links: %v", err)
	// }
	// for each link get data
	//save data

}
