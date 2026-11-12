package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPtrString(t *testing.T) {
	assert.Nil(t, PtrString(""))
	s := PtrString("hello")
	assert.NotNil(t, s)
	assert.Equal(t, "hello", *s)
}
