package scrape

import (
	"slices"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

var arbeitnowLocations = []string{
	"germany",
	"united kingdom",
	"france",
	"netherlands",
	"switzerland",
}

type Arbeitnow struct{}

func (Arbeitnow) GenerateWorkflowId() string { return "job-scrape-arbeitnow-" }

func (v Arbeitnow) GenerateWorkflowsPayload(pref models.UserPreference) []models.JobScrapeWorkflowInput {
	if strings.TrimSpace(pref.UserID) == "" || !slices.Contains(pref.Boards, "arbeitnow") {
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
			if !slices.Contains(arbeitnowLocations, location) {
				continue
			}
			searchID := utils.Slugify(keyword) + "-" + utils.Slugify(location)
			out = append(out, models.JobScrapeWorkflowInput{
				Board:      "arbeitnow",
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
