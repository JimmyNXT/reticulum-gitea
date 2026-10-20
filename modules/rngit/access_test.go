// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"testing"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/Quad4-Software/Reticulum-Go/pkg/rnsgit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	ctx := t.Context()
	s := &Server{ctx: ctx}

	owner, err := user_model.GetUserByID(ctx, 2)
	require.NoError(t, err)
	collaborator, err := user_model.GetUserByID(ctx, 40)
	require.NoError(t, err)

	publicRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, "user2", "repo1")
	require.NoError(t, err)
	privateRepo, err := repo_model.GetRepositoryByOwnerAndName(ctx, "user2", "repo2")
	require.NoError(t, err)

	for _, tc := range []struct {
		name  string
		repo  *repo_model.Repository
		user  *user_model.User
		write bool
		want  byte
	}{
		{"anonymous read public", publicRepo, nil, false, rnsgit.ResOK},
		{"anonymous read private", privateRepo, nil, false, rnsgit.ResNotFound},
		{"anonymous write public", publicRepo, nil, true, rnsgit.ResDisallowed},
		{"collaborator write public", publicRepo, collaborator, true, rnsgit.ResOK},
		{"collaborator read private", privateRepo, collaborator, false, rnsgit.ResNotFound},
		{"owner write private", privateRepo, owner, true, rnsgit.ResOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			resp := s.checkAccess(tc.repo, tc.user, tc.write)
			if tc.want == rnsgit.ResOK {
				assert.Nil(t, resp)
				return
			}
			require.NotNil(t, resp)
			assert.Equal(t, tc.want, resp[0])
		})
	}
}
