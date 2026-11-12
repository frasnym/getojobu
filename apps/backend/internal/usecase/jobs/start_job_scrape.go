package jobs

import (
	"context"
	"fmt"
	"slices"
	"sort"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

// StartJobScrape queries user preferences (filtered by params.UserIDs, or all active
// users if user-ids is not provided/empty), normalizes them into
// (board, keyword, location) -> []user_ids targets, and starts an independent
// JobScrapeWorkflow in Temporal for each target.
func (s *jobService) StartJobScrape(ctx context.Context, params models.ScrapeJobsParams) ([]string, error) {
	prefs, err := s.userPreferences.GetActivePreferences(ctx, params.UserIDs)
	if err != nil {
		return nil, fmt.Errorf("get active preferences: %w", err)
	}

	targets := normalizePreferences(prefs)
	if len(targets) == 0 {
		return []string{}, nil
	}

	workflowIDs := make([]string, 0, len(targets))
	for _, target := range targets {
		wfID := fmt.Sprintf("job-scrape-%s-%s-%s",
			utils.Slugify(target.Board),
			utils.Slugify(target.Keyword),
			utils.Slugify(target.Location),
		)

		searchID := fmt.Sprintf("%s-%s-%s",
			utils.Slugify(target.Board),
			utils.Slugify(target.Keyword),
			utils.Slugify(target.Location),
		)

		input := models.JobScrapeWorkflowInput{
			Board:      target.Board,
			Keyword:    target.Keyword,
			Location:   target.Location,
			UserIDs:    target.UserIDs,
			SearchID:   searchID,
			WorkflowID: wfID,
		}

		startedID, err := s.workflows.StartLimiterWorkItem(ctx, wfID, models.LimiterJobTypeScrape, input)
		if err != nil {
			return nil, fmt.Errorf("start scrape workflow %s: %w", wfID, err)
		}
		workflowIDs = append(workflowIDs, startedID)
	}

	return workflowIDs, nil
}

// normalizePreferences inverts and groups user preferences by search tuple:
// (board, keyword, location) -> []user_ids.
// All values are trimmed and lowercased.
// Results are deterministically sorted by Board, Keyword, Location.
// UserIDs within each target are deduplicated and sorted.
func normalizePreferences(prefs []models.UserPreference) []models.NormalizedScrapeTarget {
	type key struct {
		board    string
		keyword  string
		location string
	}

	targetsMap := make(map[key][]string)

	for _, p := range prefs {
		if !p.IsActive || strings.TrimSpace(p.UserID) == "" {
			continue
		}
		userID := strings.TrimSpace(p.UserID)

		for _, b := range p.Boards {
			board := strings.ToLower(strings.TrimSpace(b))
			if board == "" {
				continue
			}

			for _, k := range p.Keywords {
				keyword := strings.ToLower(strings.TrimSpace(k))
				if keyword == "" {
					continue
				}

				for _, l := range p.Locations {
					location := strings.ToLower(strings.TrimSpace(l))
					if location == "" {
						continue
					}

					targetKey := key{
						board:    board,
						keyword:  keyword,
						location: location,
					}
					uids := targetsMap[targetKey]
					if !slices.Contains(uids, userID) {
						targetsMap[targetKey] = append(uids, userID)
					}
				}
			}
		}
	}

	results := make([]models.NormalizedScrapeTarget, 0, len(targetsMap))
	for k, uids := range targetsMap {
		sort.Strings(uids)
		results = append(results, models.NormalizedScrapeTarget{
			Board:    k.board,
			Keyword:  k.keyword,
			Location: k.location,
			UserIDs:  uids,
		})
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].Board != results[j].Board {
			return results[i].Board < results[j].Board
		}
		if results[i].Keyword != results[j].Keyword {
			return results[i].Keyword < results[j].Keyword
		}
		return results[i].Location < results[j].Location
	})

	return results
}
