// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package integration

import (
	"net/http"
	"testing"

	auth_model "gitea.dev/models/auth"
	api "gitea.dev/modules/structs"
	"gitea.dev/tests"

	"github.com/stretchr/testify/assert"
)

func TestRNSIdentities(t *testing.T) {
	defer tests.PrepareTestEnv(t)()
	session := loginUser(t, "user2")
	token := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository)
	tokenWithUserScope := getTokenForLoggedInUser(t, session, auth_model.AccessTokenScopeWriteRepository, auth_model.AccessTokenScopeWriteUser)

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
			name: "LoggedAsUser2", makeRequest: session.MakeRequest, token: token,
			results: []int{http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden, http.StatusForbidden},
		},
		{
			name: "LoggedAsUser2WithScope", makeRequest: session.MakeRequest, token: tokenWithUserScope,
			results: []int{http.StatusOK, http.StatusOK, http.StatusOK, http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusUnprocessableEntity, http.StatusCreated, http.StatusNoContent, http.StatusNoContent},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("ListMyRNSIdentities", func(t *testing.T) {
				req := NewRequest(t, "GET", "/api/v1/user/rns_identities").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[0])
			})
			t.Run("ListRNSIdentities", func(t *testing.T) {
				req := NewRequest(t, "GET", "/api/v1/users/user2/rns_identities").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[1])
			})
			t.Run("GetRNSIdentity", func(t *testing.T) {
				req := NewRequest(t, "GET", "/api/v1/user/rns_identities/1").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[2])
			})
			t.Run("GetOtherRNSIdentity", func(t *testing.T) {
				req := NewRequest(t, "GET", "/api/v1/user/rns_identities/2").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[3])
			})
			t.Run("CreateDuplicateRNSIdentity", func(t *testing.T) {
				req := NewRequestWithJSON(t, "POST", "/api/v1/user/rns_identities", api.CreateRNSIdentityOption{
					Name:         "duplicate",
					IdentityHash: "1234567890abcdef1234567890abcdef",
				}).AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[4])
			})
			t.Run("CreateInvalidRNSIdentity", func(t *testing.T) {
				req := NewRequestWithJSON(t, "POST", "/api/v1/user/rns_identities", api.CreateRNSIdentityOption{
					Name:         "invalid",
					IdentityHash: "not-a-valid-hash",
				}).AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[5])
			})
			t.Run("CreateRNSIdentity", func(t *testing.T) {
				req := NewRequestWithJSON(t, "POST", "/api/v1/user/rns_identities", api.CreateRNSIdentityOption{
					Name:         "created-via-api",
					IdentityHash: "a3f2b8c1d4e5f60718293a4b5c6d7e8f",
				}).AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[6])
			})
			t.Run("DeleteRNSIdentity", func(t *testing.T) {
				req := NewRequest(t, "DELETE", "/api/v1/user/rns_identities/1").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[7])
			})
			t.Run("DeleteOtherRNSIdentity", func(t *testing.T) {
				req := NewRequest(t, "DELETE", "/api/v1/user/rns_identities/2").AddTokenAuth(tc.token)
				tc.makeRequest(t, req, tc.results[8])
			})
		})
	}

	t.Run("CheckState", func(t *testing.T) {
		req := NewRequest(t, "GET", "/api/v1/user/rns_identities").AddTokenAuth(tokenWithUserScope)
		resp := MakeRequest(t, req, http.StatusOK)
		rnsIdentities := DecodeJSON(t, resp, []*api.RNSIdentity{})
		assert.Len(t, rnsIdentities, 1)
		assert.Equal(t, "created-via-api", rnsIdentities[0].Name)
		assert.Equal(t, "a3f2b8c1d4e5f60718293a4b5c6d7e8f", rnsIdentities[0].IdentityHash)
		assert.Equal(t, "user2", rnsIdentities[0].Owner.UserName)

		req = NewRequest(t, "GET", "/api/v1/users/user1/rns_identities").AddTokenAuth(tokenWithUserScope)
		resp = MakeRequest(t, req, http.StatusOK)
		user1RNSIdentities := DecodeJSON(t, resp, []*api.RNSIdentity{})
		assert.Len(t, user1RNSIdentities, 1)
		assert.Equal(t, "user1-rns", user1RNSIdentities[0].Name)
	})
}
