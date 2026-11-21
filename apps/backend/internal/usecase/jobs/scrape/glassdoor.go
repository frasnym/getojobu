package scrape

import (
	"slices"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

var glassdoorLocations = allLocations

type Glassdoor struct{}

func (Glassdoor) GenerateWorkflowId() string { return "job-scrape-glassdoor-" }

func (v Glassdoor) GenerateWorkflowsPayload(pref models.UserPreference) []models.JobScrapeWorkflowInput {
	if strings.TrimSpace(pref.UserID) == "" || !slices.Contains(pref.Boards, "glassdoor") {
		return nil
	}
	var out []models.JobScrapeWorkflowInput
	for _, kw := range pref.Keywords {
		keyword := strings.ToLower(strings.TrimSpace(kw))
		if keyword == "" {
			continue
		}
		for _, loc := range pref.Locations {
			location := strings.ToLower(strings.TrimSpace(loc))
			if !slices.Contains(glassdoorLocations, location) {
				continue
			}
			searchID := utils.Slugify(keyword) + "-" + utils.Slugify(location)
			out = append(out, models.JobScrapeWorkflowInput{
				Board:      "glassdoor",
				Keyword:    keyword,
				Location:   location,
				UserIDs:    []string{pref.UserID},
				SearchID:   searchID,
				WorkflowID: v.GenerateWorkflowId() + searchID,
			})
		}
	}
	return out
}
