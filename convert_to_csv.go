package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"
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
	City     string `json:"city"`
}

func main() {
	// Get all files in current directory
	files, err := os.ReadDir(".")
	if err != nil {
		fmt.Printf("Error reading directory: %v\n", err)
		return
	}

	// Create a slice to store all rentals
	var allRentals []RentalProperty

	// Process each JSON file
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			// Read JSON file
			jsonData, err := os.ReadFile(file.Name())
			if err != nil {
				fmt.Printf("Error reading JSON file %s: %v\n", file.Name(), err)
				continue
			}

			// Parse JSON data
			var rentals []RentalProperty
			if err := json.Unmarshal(jsonData, &rentals); err != nil {
				fmt.Printf("Error parsing JSON file %s: %v\n", file.Name(), err)
				continue
			}
			// Extract commune from filename
			parts := strings.Split(file.Name(), "_")
			commune := strings.Replace(parts[2], "~", "-", -1)
			// Add commune to each rental
			for i := range rentals {
				rentals[i].City = strings.TrimSuffix(commune, ".json")
			}
			// Add rentals to the combined slice
			allRentals = append(allRentals, rentals...)
		}
	}

	// Create CSV file
	csvFile, err := os.Create("rentals.csv")
	if err != nil {
		fmt.Printf("Error creating CSV file: %v\n", err)
		return
	}
	defer csvFile.Close()

	// Create CSV writer
	writer := csv.NewWriter(csvFile)
	defer writer.Flush()

	// Write header
	headers := []string{
		"Civilite*", "Nom*", "Prénom*", "Nom de la société*", "SIRET",
		"Catégorie de tiers*", "Nature juridique*", "Adresse", "Complément",
		"Code postal *", "Commune *", "Pays *", "Téléphone *", "Téléphone 2",
		"Mail *", "Nom *", "SIRET", "Nature d'hébergement *", "Référence interne",
		"Catégorie (Atout France)*", "Date de la catégorie *",
		"Date d'application de la catégorie *", "Date de début de collecte sur la plateforme *",
		"Adresse", "Complément", "Code postal *", "Commune *", "Latitude",
		"Longitude", "Téléphone *", "Téléphone 2",
		"Nombre de chambres ou emplacements *", "Capacité (Nombre de personnes) *",
		"Site Web", "Descriptif", "Remarques",
	}
	if err := writer.Write(headers); err != nil {
		fmt.Printf("Error writing headers: %v\n", err)
		return
	}

	// Write data
	for _, rental := range allRentals {

		var name string
		if strings.HasPrefix(rental.Owner, "https://") {
			name = ""
		} else {
			name = rental.Owner
		}
		// Prepare row data
		row := []string{
			"",              // Civilite*
			name,            // Nom*
			"",              // Prénom*
			"",              // Nom de la société*
			"",              // SIRET
			"",              // Catégorie de tiers*
			"",              // Nature juridique*
			"",              // Adresse
			rental.Owner,    // Complément
			"",              // Code postal *
			"",              // Commune *
			"Guadeloupe",    // Pays *
			"",              // Téléphone *
			"",              // Téléphone 2
			"",              // Mail *
			rental.Title,    // Nom *
			"",              // SIRET
			rental.Type,     // Nature d'hébergement *
			"",              // Référence interne
			"",              // Catégorie (Atout France)*
			"",              // Date de la catégorie *
			"",              // Date d'application de la catégorie *
			"",              // Date de début de collecte sur la plateforme *
			rental.Address,  // Adresse
			"",              // Complément
			"",              // Code postal *
			rental.City,     // Commune *
			"",              // Latitude
			"",              // Longitude
			"",              // Téléphone *
			"",              // Téléphone 2
			"",              // Nombre de chambres ou emplacements *
			rental.Capacity, // Capacité (Nombre de personnes) *
			rental.URL,      // Site Web
			rental.Title,    // Descriptif
			rental.Platform, // Remarques
		}

		if err := writer.Write(row); err != nil {
			fmt.Printf("Error writing row: %v\n", err)
			return
		}
	}

	fmt.Println("CSV file has been created successfully!")
}
