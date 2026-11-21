package scrape

import (
	"slices"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

var remoteUSALocations = []string{"united states"}

type RemoteUSA struct{}

func (RemoteUSA) GenerateWorkflowId() string { return "job-scrape-remoteusa-" }

func (v RemoteUSA) GenerateWorkflowsPayload(pref models.UserPreference) []models.JobScrapeWorkflowInput {
	if strings.TrimSpace(pref.UserID) == "" || !slices.Contains(pref.Boards, "remoteusa") {
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
			if !slices.Contains(remoteUSALocations, location) {
				continue
			}
			searchID := utils.Slugify(keyword) + "-" + utils.Slugify(location)
			out = append(out, models.JobScrapeWorkflowInput{
				Board:      "remoteusa",
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
