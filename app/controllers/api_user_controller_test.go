package controllers

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatTimePtr(t *testing.T) {
	assert.Nil(t, formatTimePtr(nil))

	now := time.Date(2024, 5, 1, 12, 34, 56, 0, time.Local)
	formatted := formatTimePtr(&now)
	assert.IsType(t, "", formatted)

	expected := now.UTC().Format(time.RFC3339)
	assert.Equal(t, expected, formatted)
}
