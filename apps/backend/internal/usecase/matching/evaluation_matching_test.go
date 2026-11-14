package matching_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/frasnym/getojobu/apps/backend/internal/contract/repository"
	"github.com/frasnym/getojobu/apps/backend/internal/usecase/matching"
)

func TestNewService(t *testing.T) {
	service := matching.NewService(&repository.Contract{})
	assert.NotNil(t, service)
}
