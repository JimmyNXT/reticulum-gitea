// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckRNSIdentityHash(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{"lowercase", "abcdef1234567890abcdef1234567890", "abcdef1234567890abcdef1234567890", false},
		{"uppercase", "ABCDEF1234567890ABCDEF1234567890", "abcdef1234567890abcdef1234567890", false},
		{"too short", "abcdef1234567890abcdef123456789", "", true},
		{"too long", "abcdef1234567890abcdef1234567890a", "", true},
		{"non hex", "abcdef1234567890abcdef123456789z", "", true},
		{"trimmed", "  abcdef1234567890abcdef1234567890  ", "abcdef1234567890abcdef1234567890", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := CheckRNSIdentityHash(tt.input)
			if tt.wantErr {
				assert.ErrorIs(t, err, ErrRNSInvalidIdentityHash)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestAddRNSIdentity(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	t.Run("success", func(t *testing.T) {
		identity, err := AddRNSIdentity(t.Context(), 1, "test-key", "abcdef1234567890abcdef1234567890")
		require.NoError(t, err)
		assert.Equal(t, "test-key", identity.Name)
		assert.Equal(t, int64(1), identity.OwnerID)
	})

	t.Run("duplicate hash", func(t *testing.T) {
		_, err := AddRNSIdentity(t.Context(), 2, "other-key", "abcdef1234567890abcdef1234567890")
		assert.True(t, IsErrRNSIdentityAlreadyUsed(err))
	})

	t.Run("duplicate name", func(t *testing.T) {
		_, err := AddRNSIdentity(t.Context(), 1, "test-key", "11111111111111111111111111111111")
		assert.True(t, IsErrKeyNameAlreadyUsed(err))
	})

	t.Run("same name other owner", func(t *testing.T) {
		identity, err := AddRNSIdentity(t.Context(), 2, "test-key", "22222222222222222222222222222222")
		require.NoError(t, err)
		assert.Equal(t, int64(2), identity.OwnerID)
	})
}

func TestGetRNSIdentityForUserByID(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	identity, err := GetRNSIdentityForUserByID(t.Context(), 2, 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), identity.ID)

	_, err = GetRNSIdentityForUserByID(t.Context(), 1, 1)
	assert.True(t, IsErrKeyNotExist(err))
}

func TestDeleteRNSIdentity(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())

	identity, err := GetRNSIdentityForUserByID(t.Context(), 2, 1)
	require.NoError(t, err)

	doer := &user_model.User{ID: identity.OwnerID}
	require.NoError(t, DeleteRNSIdentity(t.Context(), doer, identity.ID))

	_, err = GetRNSIdentityForUserByID(t.Context(), 2, 1)
	assert.True(t, IsErrKeyNotExist(err))

	// deleting a non-existing identity is a no-op
	require.NoError(t, DeleteRNSIdentity(t.Context(), doer, identity.ID))
}
