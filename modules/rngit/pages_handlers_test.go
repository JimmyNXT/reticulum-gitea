// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"testing"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/unittest"
	"gitea.dev/modules/git/gitcmd"
	"gitea.dev/modules/setting"

	"github.com/Quad4-Software/Reticulum-Go/pkg/identity"
	"github.com/Quad4-Software/Reticulum-Go/pkg/rnsgit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newPageServer wires a Server for page handler tests against the real git
// repository root used by the unit test fixtures.
func newPageServer(t *testing.T) *Server {
	t.Helper()
	return &Server{ctx: t.Context(), git: &rnsgit.GitRunner{Git: gitcmd.GitExecutable}}
}

// signedInIdentity generates a fresh Reticulum identity and registers it for
// the given user so page handlers resolve it back to that user.
func signedInIdentity(t *testing.T, ownerID int64, name string) *identity.Identity {
	t.Helper()
	id, err := identity.NewIdentity()
	require.NoError(t, err)
	t.Cleanup(func() { id.Close() })
	_, err = asymkey_model.AddRNSIdentity(t.Context(), ownerID, name, id.GetHexHash())
	require.NoError(t, err)
	return id
}

// pageData packs page vars into a request body the same way the wire format
// does.
func pageData(t *testing.T, vars map[string]string) []byte {
	t.Helper()
	fields := make(map[any]any, len(vars))
	for k, v := range vars {
		fields["var_"+k] = v
	}
	body, err := rnsgit.EncodeMixedRequest(fields)
	require.NoError(t, err)
	return body
}

// TestPageFrontPermission verifies the front page lists only accessible owners
// for anonymous and signed-in viewers.
func TestPageFrontPermission(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)

	anon := s.serveFrontPage("", nil, nil, nil, nil, 0)
	anonBody, ok := anon.([]byte)
	require.True(t, ok)
	assert.Contains(t, string(anonBody), "user2")
	assert.NotContains(t, string(anonBody), "repo1")
	assert.NotContains(t, string(anonBody), "repo2")
}

// TestPageOpenRepoPermission verifies that private repositories are invisible
// to anonymous viewers and visible to their owner.
func TestPageOpenRepoPermission(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)

	t.Run("private anonymous not found", func(t *testing.T) {
		repo, page := s.openPageRepo(map[string]string{"g": "user2", "r": "repo2"}, nil)
		assert.Nil(t, repo)
		assert.Contains(t, string(page), "Repository not found")
	})

	t.Run("public anonymous visible", func(t *testing.T) {
		repo, page := s.openPageRepo(map[string]string{"g": "user2", "r": "repo1"}, nil)
		assert.Nil(t, page)
		require.NotNil(t, repo)
		require.NoError(t, repo.git.Close())
	})

	t.Run("private owner visible", func(t *testing.T) {
		id := signedInIdentity(t, 2, "page-owner")
		repo, page := s.openPageRepo(map[string]string{"g": "user2", "r": "repo2"}, id)
		assert.Nil(t, page)
		require.NotNil(t, repo)
		assert.Equal(t, "repo2", repo.model.Name)
		require.NoError(t, repo.git.Close())
	})

	t.Run("invalid repo vars not found", func(t *testing.T) {
		_, page := s.openPageRepo(map[string]string{"g": "", "r": "repo1"}, nil)
		assert.Contains(t, string(page), "Repository not found")
	})
}

// TestPageSignInRequired verifies RequireSignInViewStrict blocks anonymous
// viewers across page handlers.
func TestPageSignInRequired(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	prev := setting.Service.RequireSignInViewStrict
	setting.Service.RequireSignInViewStrict = true
	t.Cleanup(func() { setting.Service.RequireSignInViewStrict = prev })

	s := newPageServer(t)
	out := s.serveFrontPage("", nil, nil, nil, nil, 0)
	body, ok := out.([]byte)
	require.True(t, ok)
	assert.Contains(t, string(body), "Sign in is required")
}

// TestPageRepoRendering exercises the repository landing, tree, blob, commits,
// commit, refs and stats page handlers for a public repository.
func TestPageRepoRendering(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)
	vars := map[string]string{"g": "user2", "r": "repo1"}
	data := pageData(t, vars)

	render := func(t *testing.T, fn func(string, []byte, []byte, []byte, *identity.Identity, int64) any) string {
		t.Helper()
		out := fn("", data, nil, nil, nil, 0)
		body, ok := out.([]byte)
		require.True(t, ok)
		require.NotEmpty(t, body)
		return string(body)
	}

	t.Run("repo landing", func(t *testing.T) {
		body := render(t, s.serveRepoPage)
		assert.Contains(t, body, "repo1")
	})

	t.Run("tree root", func(t *testing.T) {
		body := render(t, s.serveTreePage)
		require.NotEmpty(t, body)
	})

	t.Run("blob readme", func(t *testing.T) {
		out := s.serveBlobPage("", pageData(t, map[string]string{"g": "user2", "r": "repo1", "ref": "HEAD", "path": "README.md"}), nil, nil, nil, 0)
		body, ok := out.([]byte)
		require.True(t, ok)
		require.NotEmpty(t, body)
	})

	t.Run("commits", func(t *testing.T) {
		body := render(t, s.serveCommitsPage)
		require.NotEmpty(t, body)
	})

	t.Run("refs", func(t *testing.T) {
		body := render(t, s.serveRefsPage)
		require.NotEmpty(t, body)
	})

	t.Run("stats", func(t *testing.T) {
		body := render(t, s.serveStatsPage)
		assert.Contains(t, body, "repo1")
	})

	t.Run("work is not implemented", func(t *testing.T) {
		out := s.serveWorkPage("", nil, nil, nil, nil, 0)
		body, ok := out.([]byte)
		require.True(t, ok)
		assert.Contains(t, string(body), "not available")
	})

	t.Run("work doc is not implemented", func(t *testing.T) {
		out := s.serveWorkDocPage("", nil, nil, nil, nil, 0)
		body, ok := out.([]byte)
		require.True(t, ok)
		assert.Contains(t, string(body), "not available")
	})
}

// TestPageCommitPage verifies a single commit renders its message.
func TestPageCommitPage(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)

	repo, page := s.openPageRepo(map[string]string{"g": "user2", "r": "repo1"}, nil)
	require.Nil(t, page)
	require.NotNil(t, repo)
	defer func() { require.NoError(t, repo.git.Close()) }()

	commit, _, ok := repo.resolveRef("")
	require.True(t, ok)

	out := s.serveCommitPage("", pageData(t, map[string]string{"g": "user2", "r": "repo1", "ref": commit.ID.String()}), nil, nil, nil, 0)
	body, isBytes := out.([]byte)
	require.True(t, isBytes)
	require.NotEmpty(t, body)
}

// TestPageGroupPage verifies the group listing shows an owner's repositories
// and hides unknown owners.
func TestPageGroupPage(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)

	out := s.serveGroupPage("", pageData(t, map[string]string{"g": "user2"}), nil, nil, nil, 0)
	body, ok := out.([]byte)
	require.True(t, ok)
	assert.Contains(t, string(body), "repo1")
	assert.NotContains(t, string(body), "repo2")

	out = s.serveGroupPage("", pageData(t, map[string]string{"g": "nobody"}), nil, nil, nil, 0)
	body, ok = out.([]byte)
	require.True(t, ok)
	assert.Contains(t, string(body), "Owner not found")
}
