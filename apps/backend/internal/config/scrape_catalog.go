package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type ScrapeCatalog struct {
	Locations []ScrapeLocation `json:"locations"`
	Boards    []ScrapeBoard    `json:"boards"`
}

type ScrapeLocation struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

type ScrapeBoard struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

func LoadScrapeCatalog(path string) (*ScrapeCatalog, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scrape catalog %s: %w", path, err)
	}
	var catalog ScrapeCatalog
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, fmt.Errorf("parse scrape catalog %s: %w", path, err)
	}
	return &catalog, nil
}
