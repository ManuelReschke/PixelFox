package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserSettingsIssueAPIKey(t *testing.T) {
	us := &UserSettings{UserID: 1}

	key, err := us.IssueAPIKey()
	require.NoError(t, err)
	require.NotEmpty(t, key)

	assert.NotEmpty(t, us.APIKeyHash)
	assert.NotEmpty(t, us.APIKeyPrefix)
	assert.NotNil(t, us.APIKeyCreatedAt)
	assert.Nil(t, us.APIKeyLastUsedAt)
	assert.True(t, us.HasActiveAPIKey())
	assert.Equal(t, HashAPIKey(key), us.APIKeyHash)
}

func TestUserSettingsRevokeAPIKey(t *testing.T) {
	us := &UserSettings{UserID: 99}
	_, err := us.IssueAPIKey()
	require.NoError(t, err)

	us.RevokeAPIKey()

	assert.False(t, us.HasActiveAPIKey())
	assert.Equal(t, "", us.APIKeyHash)
	assert.Equal(t, "", us.APIKeyPrefix)
	assert.NotNil(t, us.APIKeyRevokedAt)
}
