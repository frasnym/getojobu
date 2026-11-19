package matching

import (
	"context"
	"fmt"
	"strings"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

// composeFacts builds the contact/role/education summary the resume and cover
// letter workers need. It also attaches raw education rows to a copy of the
// candidate profile. Both the worker GET and the refresh endpoint use it, so
// cached data has the same shape as the previous enqueue payload.
func (s *matchingService) composeFacts(
	ctx context.Context,
	user *models.User,
	candidate map[string]interface{},
) (map[string]interface{}, map[string]interface{}) {
	if candidate == nil {
		candidate = map[string]interface{}{}
	}

	var educationLine string
	var eduRows []*models.UserEducation
	if s.eduRepo != nil {
		edus, err := s.eduRepo.ListByUser(ctx, user.ID)
		if err == nil && len(edus) > 0 {
			eduRows = edus
			var lines []string
			for _, top := range edus {
				var yearStr string
				if top.StartYear != nil && top.EndYear != nil {
					yearStr = fmt.Sprintf("%d-%d", *top.StartYear, *top.EndYear)
				} else if top.EndYear != nil {
					yearStr = fmt.Sprintf("%d", *top.EndYear)
				} else if top.StartYear != nil {
					yearStr = fmt.Sprintf("%d", *top.StartYear)
				}
				parts := []string{fmt.Sprintf("%s in %s, %s", top.Degree, top.FieldOfStudy, top.Institution)}
				if yearStr != "" {
					parts = append(parts, yearStr)
				}
				if top.GPA != nil && *top.GPA != "" {
					parts = append(parts, fmt.Sprintf("GPA: %s", *top.GPA))
				}
				lines = append(lines, strings.Join(parts, " | "))
			}
			educationLine = strings.Join(lines, "\n")
		}
	}

	var roles []map[string]interface{}
	if s.expRepo != nil {
		exps, err := s.expRepo.ListByUser(ctx, user.ID)
		if err == nil {
			for _, exp := range exps {
				period := ""
				if !exp.StartedAt.IsZero() {
					start := exp.StartedAt.Format("Jan 2006")
					end := "Current"
					if !exp.EndedAt.IsZero() {
						end = exp.EndedAt.Format("Jan 2006")
					}
					period = fmt.Sprintf("%s - %s", start, end)
				}
				roles = append(roles, map[string]interface{}{
					"id":      exp.ID,
					"company": exp.Organization,
					"role":    exp.RoleTitle,
					"period":  period,
				})
			}
		}
	}
	if len(roles) == 0 {
		if rawExps, ok := candidate["experience"].([]interface{}); ok {
			for idx, rawExp := range rawExps {
				if expMap, ok := rawExp.(map[string]interface{}); ok {
					roles = append(roles, map[string]interface{}{
						"id":      fmt.Sprintf("exp-%d", idx+1),
						"company": expMap["company"],
						"role":    expMap["role"],
						"period":  expMap["period"],
					})
				}
			}
		}
	}

	var contactParts []string
	for _, value := range []*string{user.FullName, user.Phone, user.Email, user.LinkedInURL, user.GitHubURL} {
		if value != nil && strings.TrimSpace(*value) != "" {
			contactParts = append(contactParts, *value)
		}
	}

	enriched := make(map[string]interface{}, len(candidate)+1)
	for key, value := range candidate {
		enriched[key] = value
	}
	enriched["educations"] = eduRows

	facts := map[string]interface{}{
		"name":      utils.PtrStringOrEmpty(user.FullName),
		"headline":  utils.PtrStringOrEmpty(user.Headline),
		"phone":     utils.PtrStringOrEmpty(user.Phone),
		"email":     utils.PtrStringOrEmpty(user.Email),
		"linkedin":  utils.PtrStringOrEmpty(user.LinkedInURL),
		"github":    utils.PtrStringOrEmpty(user.GitHubURL),
		"portfolio": utils.PtrStringOrEmpty(user.PortfolioURL),
		"education": educationLine,
		"contact":   strings.Join(contactParts, " | "),
		"roles":     roles,
	}
	return facts, enriched
}
