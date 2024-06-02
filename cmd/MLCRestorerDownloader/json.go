package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type TitleMap struct {
	MLC map[string][]string `json:"MLC"`
	SLC map[string][]string `json:"SLC"`
}

func readTitleInfoFromFile(filename string) (TitleMap, error) {
	titles := TitleMap{}
	jsonData, err := os.ReadFile(filename)
	if err != nil {
		return titles, fmt.Errorf("error reading file: %w", err)
	}

	if err := json.Unmarshal(jsonData, &titles); err != nil {
		return titles, fmt.Errorf("error parsing JSON: %w", err)
	}

	return titles, nil
}
