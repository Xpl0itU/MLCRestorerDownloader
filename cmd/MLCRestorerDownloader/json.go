package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type TitleMap struct {
	MLC map[string][]string `json:"MLC"`
	SLC map[string][]string `json:"SLC"`
}

func readTitleInfoFromFile(filename string) (TitleMap, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return TitleMap{}, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Read the file contents
	jsonData, err := io.ReadAll(file)
	if err != nil {
		return TitleMap{}, fmt.Errorf("error reading file: %w", err)
	}

	// Define a map to hold the parsed data
	titles := TitleMap{}

	// Parse the JSON into the titles map
	err = json.Unmarshal(jsonData, &titles)
	if err != nil {
		return TitleMap{}, fmt.Errorf("error parsing JSON: %w", err)
	}

	return titles, nil
}
