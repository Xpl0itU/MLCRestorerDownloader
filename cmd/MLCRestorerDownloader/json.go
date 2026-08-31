package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type TitleMap struct {
	MLC map[string][]string `json:"MLC"`
	SLC map[string][]string `json:"SLC"`
}

func readTitleInfoFromFile(filename string) (TitleMap, error) {
	titles := TitleMap{}

	filePath := resolveFilePath(filename)
	jsonData, err := os.ReadFile(filePath)
	if err != nil {
		return titles, fmt.Errorf("error reading file: %w", err)
	}

	if err := json.Unmarshal(jsonData, &titles); err != nil {
		return titles, fmt.Errorf("error parsing JSON: %w", err)
	}

	return titles, nil
}

func resolveFilePath(filename string) string {
	if _, err := os.Stat(filename); err == nil {
		return filename
	}

	if executable, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(executable), filename)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return filename
}
