package utils

import (
	"database/sql"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNullString(t *testing.T) {
	assert.Equal(t, "", NullString(sql.NullString{Valid: false}))
	assert.Equal(t, "hello", NullString(sql.NullString{String: "hello", Valid: true}))
}

func TestNullStringPtr(t *testing.T) {
	assert.Nil(t, NullStringPtr(sql.NullString{Valid: false}))
	s := NullStringPtr(sql.NullString{String: "abc", Valid: true})
	assert.NotNil(t, s)
	assert.Equal(t, "abc", *s)
}

func TestNullBoolPtr(t *testing.T) {
	assert.Nil(t, NullBoolPtr(sql.NullBool{Valid: false}))
	b := NullBoolPtr(sql.NullBool{Bool: true, Valid: true})
	assert.NotNil(t, b)
	assert.True(t, *b)
}

func TestNullTime(t *testing.T) {
	assert.True(t, NullTime(sql.NullTime{Valid: false}).IsZero())
	now := time.Now()
	assert.Equal(t, now, NullTime(sql.NullTime{Time: now, Valid: true}))
}

func TestNullTimePtr(t *testing.T) {
	assert.Nil(t, NullTimePtr(sql.NullTime{Valid: false}))
	now := time.Now()
	tm := NullTimePtr(sql.NullTime{Time: now, Valid: true})
	assert.NotNil(t, tm)
	assert.Equal(t, now, *tm)
}

func TestTimeOrNil(t *testing.T) {
	assert.Nil(t, TimeOrNil(time.Time{}))
	now := time.Now()
	assert.Equal(t, now, TimeOrNil(now))
}
