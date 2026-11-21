package jobs

import (
	"context"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

func (s *jobService) StartJobScrape(ctx context.Context, params models.ScrapeJobsParams) ([]string, error) {
	prefs, err := s.userPreferences.GetActivePreferences(ctx, params.UserIDs)
	if err != nil {
		return nil, fmt.Errorf("get active preferences: %w", err)
	}

	byID := make(map[string]*models.JobScrapeWorkflowInput)
	for _, pref := range prefs {
		for _, board := range pref.Boards {
			vendor, err := s.registry.GetVendorByName(board)
			if err != nil {
				continue
			}
			for _, payload := range vendor.GenerateWorkflowsPayload(pref) {
				p := payload
				if existing, ok := byID[p.WorkflowID]; ok {
					existing.UserIDs = append(existing.UserIDs, p.UserIDs...)
					continue
				}
				byID[p.WorkflowID] = &p
			}
		}
	}

	started := make([]string, 0, len(byID))
	for wfID, payload := range byID {
		id, err := s.workflows.StartLimiterWorkItem(ctx, wfID, models.LimiterJobTypeScrape, *payload)
		if err != nil {
			return nil, fmt.Errorf("start scrape workflow %s: %w", wfID, err)
		}
		started = append(started, id)
	}
	return started, nil
}
