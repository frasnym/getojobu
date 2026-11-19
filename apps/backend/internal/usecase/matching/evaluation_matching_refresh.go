package matching

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

// RefreshProfileCache recomposes and stores the current candidate profile and
// facts for the given users. A machine caller triggers this after profile
// updates; worker GETs read the cached columns.
func (s *matchingService) RefreshProfileCache(
	ctx context.Context,
	userIDs []uuid.UUID,
) ([]*models.RefreshProfileResult, error) {
	results := make([]*models.RefreshProfileResult, 0, len(userIDs))
	for _, id := range userIDs {
		result := &models.RefreshProfileResult{UserID: id.String()}

		user, err := s.usersRepo.FindByID(ctx, id.String())
		if err != nil {
			msg := err.Error()
			result.Error = &msg
			results = append(results, result)
			continue
		}
		candidate, err := s.composeCandidateProfile(ctx, id.String())
		if err != nil {
			msg := err.Error()
			result.Error = &msg
			results = append(results, result)
			continue
		}
		facts, enriched := s.composeFacts(ctx, user, candidate)
		profileJSON, _ := json.Marshal(enriched)
		factsJSON, _ := json.Marshal(facts)
		if err := s.usersRepo.UpdateProfileCache(ctx, id.String(), profileJSON, factsJSON); err != nil {
			msg := err.Error()
			result.Error = &msg
			results = append(results, result)
			continue
		}

		results = append(results, result)
	}
	return results, nil
}
