package scrape

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/frasnym/getojobu/apps/backend/internal/models"
)

func TestGetVendorByName(t *testing.T) {
	registry := NewJobScrapeRegistry()

	vendor, err := registry.GetVendorByName("glassdoor")
	require.NoError(t, err)
	assert.Equal(t, "job-scrape-glassdoor-", vendor.GenerateWorkflowId())

	_, err = registry.GetVendorByName("unknown")
	require.Error(t, err)
}

func TestLinkedInPayload(t *testing.T) {
	vendor := NewJobScrapeRegistry().vendors["linkedin"]
	payloads := vendor.GenerateWorkflowsPayload(models.UserPreference{
		UserID:    "userA",
		Keywords:  []string{" GoLang "},
		Locations: []string{" INDONESIA "},
		Boards:    []string{"linkedin"},
	})

	require.Len(t, payloads, 1)
	assert.Equal(t, models.JobScrapeWorkflowInput{
		Board:      "linkedin",
		Keyword:    "golang",
		Location:   "indonesia",
		UserIDs:    []string{"userA"},
		SearchID:   "golang-indonesia",
		WorkflowID: "job-scrape-linkedin-golang-indonesia",
	}, payloads[0])
}

func TestRemoteUSALocationCheck(t *testing.T) {
	vendor := NewJobScrapeRegistry().vendors["remoteusa"]

	assert.Empty(t, vendor.GenerateWorkflowsPayload(models.UserPreference{
		UserID:    "userA",
		Keywords:  []string{"golang"},
		Locations: []string{"indonesia"},
		Boards:    []string{"remoteusa"},
	}))

	payloads := vendor.GenerateWorkflowsPayload(models.UserPreference{
		UserID:    "userA",
		Keywords:  []string{"golang"},
		Locations: []string{"united states"},
		Boards:    []string{"remoteusa"},
	})
	require.Len(t, payloads, 1)
	assert.Equal(t, "job-scrape-remoteusa-golang-united-states", payloads[0].WorkflowID)
}

func TestArbeitnowLocationCheck(t *testing.T) {
	vendor := NewJobScrapeRegistry().vendors["arbeitnow"]

	assert.Empty(t, vendor.GenerateWorkflowsPayload(models.UserPreference{
		UserID:    "userA",
		Keywords:  []string{"golang"},
		Locations: []string{"indonesia"},
		Boards:    []string{"arbeitnow"},
	}))

	payloads := vendor.GenerateWorkflowsPayload(models.UserPreference{
		UserID:    "userA",
		Keywords:  []string{"golang"},
		Locations: []string{"germany"},
		Boards:    []string{"arbeitnow"},
	})
	require.Len(t, payloads, 1)
	assert.Equal(t, "job-scrape-arbeitnow-golang-germany", payloads[0].WorkflowID)
}

func TestVendorIgnoresOtherBoards(t *testing.T) {
	vendor := NewJobScrapeRegistry().vendors["linkedin"]
	assert.Empty(t, vendor.GenerateWorkflowsPayload(models.UserPreference{
		UserID:    "userA",
		Keywords:  []string{"golang"},
		Locations: []string{"indonesia"},
		Boards:    []string{"glassdoor"},
	}))
}
