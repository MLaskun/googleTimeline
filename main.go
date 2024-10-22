package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Structs to match the JSON structure
type Location struct {
	LatitudeE7  int64 `json:"latitudeE7"`
	LongitudeE7 int64 `json:"longitudeE7"`
}

type ActivitySegment struct {
	StartLocation Location `json:"startLocation"`
	EndLocation   Location `json:"endLocation"`
	Distance      int      `json:"distance"` // Distance in meters
	Duration      struct {
		StartTimestamp string `json:"startTimestamp"`
	} `json:"duration"`
}

type TimelineObject struct {
	ActivitySegment *ActivitySegment `json:"activitySegment,omitempty"`
}

type TimelineData struct {
	TimelineObjects []TimelineObject `json:"timelineObjects"`
}

// Helper function to convert E7 lat/lng to float64
func toFloatCoord(coordE7 int64) float64 {
	return float64(coordE7) / 1e7
}

// Function to call Nominatim API and get country code from latitude/longitude
func getCountryCode(lat float64, lng float64) (string, error) {
	baseURL := "https://nominatim.openstreetmap.org/reverse"

	// Build the query URL
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	params := url.Values{}
	params.Add("lat", fmt.Sprintf("%f", lat))
	params.Add("lon", fmt.Sprintf("%f", lng))
	params.Add("format", "json")
	params.Add("zoom", "3") // Level 3 is for countries
	u.RawQuery = params.Encode()

	// Make the HTTP request
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(u.String())
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// Parse the response
	var result struct {
		Address struct {
			CountryCode string `json:"country_code"`
		} `json:"address"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return "", err
	}

	// Return the country code
	if result.Address.CountryCode != "" {
		return result.Address.CountryCode, nil
	}

	return "", fmt.Errorf("no country found for lat: %f, lng: %f", lat, lng)
}

func main() {
	// Load the JSON data
	filename := "timeline.json" // Change to the actual file name
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error reading JSON file: %v\n", err)
		os.Exit(1)
	}

	// Parse the JSON data
	var timeline TimelineData
	err = json.Unmarshal(data, &timeline)
	if err != nil {
		fmt.Printf("Error parsing JSON file: %v\n", err)
		os.Exit(1)
	}

	// Create a map to store total distance per country per day
	countryDistancesByDay := make(map[string]map[string]float64)

	// Iterate through the timeline objects
	for _, obj := range timeline.TimelineObjects {
		if obj.ActivitySegment != nil {
			startLat := toFloatCoord(obj.ActivitySegment.StartLocation.LatitudeE7)
			startLng := toFloatCoord(obj.ActivitySegment.StartLocation.LongitudeE7)

			// Get the country code for the start location using Nominatim API
			startCountry, err := getCountryCode(startLat, startLng)
			if err != nil {
				fmt.Printf("Error getting country for lat: %f, lng: %f - %v\n", startLat, startLng, err)
				continue
			}

			// Parse the startTimestamp to extract the date
			startTime, err := time.Parse(time.RFC3339, obj.ActivitySegment.Duration.StartTimestamp)
			if err != nil {
				fmt.Printf("Error parsing startTimestamp: %v\n", err)
				continue
			}
			date := startTime.Format("2006-01-02") // Format date as YYYY-MM-DD

			// Initialize map for the country if not already initialized
			if countryDistancesByDay[startCountry] == nil {
				countryDistancesByDay[startCountry] = make(map[string]float64)
			}

			// Add the distance to the respective country and date
			countryDistancesByDay[startCountry][date] += float64(obj.ActivitySegment.Distance) / 1000.0
		}
	}

	// Create a filename with today's date
	today := time.Now().Format("2006-01-02") // Format as YYYY-MM-DD
	outputFilename := fmt.Sprintf("distance_report_%s.txt", today)

	// Open the file for writing
	file, err := os.Create(outputFilename)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		os.Exit(1)
	}
	defer file.Close()

	// Write the total distance traveled per country per day to the file
	file.WriteString("Total distance traveled in each country per day (in kilometers):\n")
	for country, dateDistances := range countryDistancesByDay {
		file.WriteString(fmt.Sprintf("Country: %s\n", country))
		for date, distance := range dateDistances {
			file.WriteString(fmt.Sprintf("  %s: %.2f km\n", date, distance))
		}
	}

	fmt.Printf("Distance report saved to %s\n", outputFilename)
}

