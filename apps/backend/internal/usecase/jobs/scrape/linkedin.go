package scrape

import (
	"slices"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

var linkedInLocations = allLocations

type LinkedIn struct{}

func (LinkedIn) GenerateWorkflowId() string { return "job-scrape-linkedin-" }

func (v LinkedIn) GenerateWorkflowsPayload(pref models.UserPreference) []models.JobScrapeWorkflowInput {
	if strings.TrimSpace(pref.UserID) == "" || !slices.Contains(pref.Boards, "linkedin") {
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
			if !slices.Contains(linkedInLocations, location) {
				continue
			}
			searchID := utils.Slugify(keyword) + "-" + utils.Slugify(location)
			out = append(out, models.JobScrapeWorkflowInput{
				Board:      "linkedin",
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
