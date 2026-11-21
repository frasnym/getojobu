package scrape

import (
	"fmt"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

// allLocations is the location rule shared by vendors that operate globally.
var allLocations = []string{
	"austria",
	"australia",
	"belgium",
	"canada",
	"denmark",
	"finland",
	"france",
	"germany",
	"indonesia",
	"ireland",
	"japan",
	"malaysia",
	"netherlands",
	"new zealand",
	"norway",
	"poland",
	"portugal",
	"singapore",
	"spain",
	"sweden",
	"switzerland",
	"united arab emirates",
	"united kingdom",
	"united states",
}

type JobScrapeVendor interface {
	GenerateWorkflowId() string
	GenerateWorkflowsPayload(pref models.UserPreference) []models.JobScrapeWorkflowInput
}

type JobScrapeRegistry struct {
	vendors map[string]JobScrapeVendor
}

func NewJobScrapeRegistry() *JobScrapeRegistry {
	return &JobScrapeRegistry{
		vendors: map[string]JobScrapeVendor{
			"linkedin":  LinkedIn{},
			"glassdoor": Glassdoor{},
			"arbeitnow": Arbeitnow{},
			"remoteusa": RemoteUSA{},
		},
	}
}

func (r *JobScrapeRegistry) GetVendorByName(name string) (JobScrapeVendor, error) {
	vendor, exists := r.vendors[name]
	if !exists {
		return nil, fmt.Errorf("vendor not found: %s", name)
	}
	return vendor, nil
}
