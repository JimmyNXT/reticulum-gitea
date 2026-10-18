// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"strconv"
	"testing"

	auth_model "gitea.dev/models/auth"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestRNSKeys(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	tokenWithRepoScope := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	tokenWithUserScope := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteUser)

	tt := []struct {
		name        string
		makeRequest makeRequestFunc
		token       string
		results     []int
	}{
		{
			name: "NoLogin", makeRequest: MakeRequest, token: "",
			results: []int{http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized, http.StatusUnauthorized},
		},
		{
			name: "LoggedAsUser2WrongScope", makeRequest: session.MakeRequest, token: tokenWithRepoScope,
			results: []int{http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden},
		},
		{
			name: "LoggedAsUser2WithScope", makeRequest: session.MakeRequest, token: tokenWithUserScope,
			results: []int{http.StatusOK, http.StatusOK, http.StatusOK, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, http.StatusCreated, http.StatusNoContent, http.StatusNoContent},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("ListMyRNSKeys", func(t *testing.T) {
				req := NewRequest(t, "GET", "/api/v1/user/rns_keys").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[0])
			})
			t.Run("ListRNSKeys", func(t *testing.T) {
				req := NewRequest(t, "GET", "/api/v1/users/user2/rns_keys").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[1])
			})
			t.Run("GetOwnRNSKey", func(t *testing.T) {
				req := NewRequest(t, "GET", "/api/v1/user/rns_keys/1").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[2])
			})
			t.Run("GetNotOwnedRNSKey", func(t *testing.T) {
				req := NewRequest(t, "GET", "/api/v1/user/rns_keys/2").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[3])
			})
			t.Run("CreateDuplicateHashRNSKey", func(t *testing.T) {
				req := NewRequestWithJSON(t, "POST", "/api/v1/user/rns_keys", api.CreateRNSKeyOption{
					Name:         "dup-hash",
					IdentityHash: "1234567890abcdef1234567890abcdef",
				}).AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[4])
			})
			t.Run("CreateInvalidHashRNSKey", func(t *testing.T) {
				req := NewRequestWithJSON(t, "POST", "/api/v1/user/rns_keys", api.CreateRNSKeyOption{
					Name:         "invalid-hash",
					IdentityHash: "not-a-valid-hash",
				}).AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[5])
			})
			t.Run("CreateValidRNSKey", func(t *testing.T) {
				req := NewRequestWithJSON(t, "POST", "/api/v1/user/rns_keys", api.CreateRNSKeyOption{
					Name:         "created-via-api",
					IdentityHash: "a3f2b8c1d4e5f60718293a4b5c6d7e8f",
				}).AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[6])
			})
			t.Run("DeleteOwnRNSKey", func(t *testing.T) {
				req := NewRequest(t, "DELETE", "/api/v1/user/rns_keys/1").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[7])
			})
			t.Run("DeleteNotOwnedRNSKey", func(t *testing.T) {
				req := NewRequest(t, "DELETE", "/api/v1/user/rns_keys/2").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[8])
			})
		})
	}

	t.Run("CheckState", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/user/rns_keys").AddTokenAuth(tokenWithUserScope)
		resp := MakeRequest(t, req, http.StatusOK)
		keys := DecodeJSON(t, resp, []*api.RNSKey{})
		assert.Len(t, keys, 1)
		assert.Equal(t, "created-via-api", keys[0].Name)
		assert.Equal(t, "a3f2b8c1d4e5f60718293a4b5c6d7e8f", keys[0].IdentityHash)
		assert.Equal(t, "user2", keys[0].Owner.UserName)

		req = NewRequest(t, "GET", "/api/v1/user/rns_keys/"+strconv.FormatInt(keys[0].ID, 10)).AddTokenAuth(tokenWithUserScope)
		resp = MakeRequest(t, req, http.StatusOK)
		key := DecodeJSON(t, resp, &api.RNSKey{})
		assert.Equal(t, keys[0].IdentityHash, key.IdentityHash)

		req = NewRequest(t, "GET", "/api/v1/users/user1/rns_keys").AddTokenAuth(tokenWithUserScope)
		resp = MakeRequest(t, req, http.StatusOK)
		keys = DecodeJSON(t, resp, []*api.RNSKey{})
		assert.Len(t, keys, 1)
		assert.Equal(t, "user1-rns", keys[0].Name)
	})
}
