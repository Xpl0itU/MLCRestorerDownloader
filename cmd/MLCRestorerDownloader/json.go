package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

type TitleMap map[string][]string

func readTitleInfoFromFile(filename string) (TitleMap, error) {
	// Open the file
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Read the file contents
	jsonData, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading file: %w", err)
	}

	// Define a map to hold the parsed data
	titles := make(TitleMap)

	// Parse the JSON into the titles map
	err = json.Unmarshal(jsonData, &titles)
	if err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	return titles, nil
}
