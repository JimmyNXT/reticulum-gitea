// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package asymkey

import (
	"testing"

	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
)

func TestCheckRNSIdentityHash(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	cases := []struct {
		hash    string
		want    string
		wantErr bool
	}{
		{"1234567890abcdef1234567890abcdef", "1234567890abcdef1234567890abcdef", false},
		{"1234567890ABCDEF1234567890ABCDEF", "1234567890abcdef1234567890abcdef", false},
		{"1234567890abcdef1234567890abcde", "", true},
		{"1234567890abcdef1234567890abcdef0", "", true},
		{"1234567890abcdef1234567890abcdeg", "", true},
		{" 1234567890abcdef1234567890abcdef ", "1234567890abcdef1234567890abcdef", false},
	}

	for _, c := range cases {
		got, err := CheckRNSIdentityHash(c.hash)
		if c.wantErr {
			assert.Error(t, err)
		} else {
			assert.NoError(t, err)
			assert.Equal(t, c.want, got)
		}
	}
}

func TestAddRNSKey(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	key, err := AddRNSKey(t.Context(), 1, "test-key", "abcdef1234567890abcdef1234567890")
	assert.NoError(t, err)
	assert.Equal(t, "test-key", key.Name)
	assert.Equal(t, int64(1), key.OwnerID)

	_, err = AddRNSKey(t.Context(), 2, "other-key", "abcdef1234567890abcdef1234567890")
	assert.Error(t, err)
	assert.True(t, IsErrRNSIdentityAlreadyUsed(err))

	_, err = AddRNSKey(t.Context(), 1, "test-key", "11111111111111111111111111111111")
	assert.Error(t, err)
	assert.True(t, IsErrKeyNameAlreadyUsed(err))

	key2, err := AddRNSKey(t.Context(), 2, "test-key", "22222222222222222222222222222222")
	assert.NoError(t, err)
	assert.Equal(t, "test-key", key2.Name)
}

func TestGetRNSKeyForUserByID(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	key, err := GetRNSKeyForUserByID(t.Context(), 2, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), key.ID)

	_, err = GetRNSKeyForUserByID(t.Context(), 1, 1)
	assert.Error(t, err)
	assert.True(t, IsErrKeyNotExist(err))
}

func TestDeleteRNSKey(t *testing.T) {
	assert.NoError(t, unittest.PrepareTestDatabase())

	doer := &user_model.User{ID: 2}
	err := DeleteRNSKey(t.Context(), doer, 1)
	assert.NoError(t, err)

	doer2 := &user_model.User{ID: 1}
	err = DeleteRNSKey(t.Context(), doer2, 1)
	assert.NoError(t, err)
}
