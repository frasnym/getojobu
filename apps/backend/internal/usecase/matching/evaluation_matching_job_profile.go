package matching

import (
	"encoding/json"
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
	"github.com/frasnym/getojobu/apps/backend/internal/utils"
)

// DeriveJobProfile builds the worker-facing job profile from the shared job
// listing. Service metadata is stripped and title/company/location are copied
// from the listing, so the result is the same wherever it is built.
func DeriveJobProfile(job *models.Job) (map[string]interface{}, error) {
	var profile map[string]interface{}
	if err := json.Unmarshal(utils.StripJSONKeys(job.AnalyzeResult, "metadata"), &profile); err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, fmt.Errorf("job %q analyze_result is not a JSON object", job.Slug)
	}
	profile["title"] = job.Title
	profile["company"] = job.Company
	profile["location"] = job.Location
	family, _ := profile["family"].(string)
	if family == "" {
		family = "other"
		profile["family"] = family
	}
	return profile, nil
}
