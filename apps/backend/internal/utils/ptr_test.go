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

func TestPtrFloatOrZero(t *testing.T) {
	assert.Equal(t, float64(0), PtrFloatOrZero(nil))
	v := 42.5
	assert.Equal(t, 42.5, PtrFloatOrZero(&v))
}

func TestPtrStringOrEmpty(t *testing.T) {
	assert.Equal(t, "", PtrStringOrEmpty(nil))
	s := "hello"
	assert.Equal(t, "hello", PtrStringOrEmpty(&s))
}
