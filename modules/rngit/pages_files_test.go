// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"testing"

	"gitea.dev/models/unittest"

	"github.com/Quad4-Software/Reticulum-Go/pkg/link"
	"github.com/Quad4-Software/Reticulum-Go/pkg/rnsgit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPublishedReleasesAndByTag verifies draft filtering and tag lookup.
func TestPublishedReleasesAndByTag(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)

	releases := s.publishedReleases(1)
	require.NotEmpty(t, releases)
	for _, rel := range releases {
		assert.False(t, rel.IsDraft)
	}
	assert.Nil(t, s.releaseByTag("draft-release", releases))
	assert.NotNil(t, s.releaseByTag("v1.1", releases))
	assert.Nil(t, s.releaseByTag("no-such-tag", releases))
}

// TestServeReleasesPage verifies the release list page hides drafts and rejects
// invisible repositories.
func TestServeReleasesPage(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)

	out := s.serveReleasesPage("", pageData(t, map[string]string{"g": "user2", "r": "repo1"}), nil, nil, nil, 0)
	body, ok := out.([]byte)
	require.True(t, ok)
	assert.Contains(t, string(body), "v1.1")
	assert.NotContains(t, string(body), "draft-release")

	out = s.serveReleasesPage("", pageData(t, map[string]string{"g": "user2", "r": "repo2"}), nil, nil, nil, 0)
	body, ok = out.([]byte)
	require.True(t, ok)
	assert.Contains(t, string(body), "Repository not found")
}

// TestServeReleasePage verifies release notes rendering and guard rails.
func TestServeReleasePage(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)

	t.Run("by tag", func(t *testing.T) {
		out := s.serveReleasePage("", pageData(t, map[string]string{"g": "user2", "r": "repo1", "t": "v1.1"}), nil, nil, nil, 0)
		body, ok := out.([]byte)
		require.True(t, ok)
		assert.Contains(t, string(body), "v1.1")
	})

	t.Run("latest", func(t *testing.T) {
		out := s.serveReleasePage("", pageData(t, map[string]string{"g": "user2", "r": "repo1", "t": "latest"}), nil, nil, nil, 0)
		body, ok := out.([]byte)
		require.True(t, ok)
		assert.NotContains(t, string(body), "Release not found")
	})

	t.Run("unknown tag", func(t *testing.T) {
		out := s.serveReleasePage("", pageData(t, map[string]string{"g": "user2", "r": "repo1", "t": "nope"}), nil, nil, nil, 0)
		body, ok := out.([]byte)
		require.True(t, ok)
		assert.Contains(t, string(body), "Release not found")
	})

	t.Run("invalid tag", func(t *testing.T) {
		out := s.serveReleasePage("", pageData(t, map[string]string{"g": "user2", "r": "repo1", "t": "../x"}), nil, nil, nil, 0)
		body, ok := out.([]byte)
		require.True(t, ok)
		assert.Contains(t, string(body), "Invalid release")
	})
}

// TestServeArtifact verifies a missing attachment streams nothing.
func TestServeArtifact(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)

	out := s.serveArtifact("", pageData(t, map[string]string{"g": "user2", "r": "repo1", "t": "v1.1", "a": "missing.zip"}), nil, nil, nil, 0)
	assert.Nil(t, out)

	out = s.serveArtifact("", pageData(t, map[string]string{"g": "user2", "r": "repo2", "t": "v1.1", "a": "x"}), nil, nil, nil, 0)
	assert.Nil(t, out)

	out = s.serveArtifact("", pageData(t, map[string]string{"g": "user2", "r": "repo1", "t": "v1.1", "a": "../evil"}), nil, nil, nil, 0)
	assert.Nil(t, out)
}

// TestServeDownload verifies file download and markdown to micron conversion
// plus path traversal rejection.
func TestServeDownload(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)

	findFile := func(t *testing.T) (owner, repo, file string) {
		return "user2", "repo1", "README.md"
	}

	t.Run("raw text file", func(t *testing.T) {
		owner, repo, file := findFile(t)
		out := s.serveDownload("", pageData(t, map[string]string{"g": owner, "r": repo, "ref": "HEAD", "path": file}), nil, nil, nil, 0)
		fr, ok := out.(link.FileResponse)
		require.True(t, ok)
		require.NotEmpty(t, fr.Data)
	})

	t.Run("markdown as micron", func(t *testing.T) {
		owner, repo, file := findFile(t)
		out := s.serveDownload("", pageData(t, map[string]string{"g": owner, "r": repo, "ref": "HEAD", "path": file, "fmt": "mu"}), nil, nil, nil, 0)
		fr, ok := out.(link.FileResponse)
		require.True(t, ok)
		require.NotEmpty(t, fr.Data)
	})

	t.Run("path traversal rejected", func(t *testing.T) {
		owner, repo, _ := findFile(t)
		out := s.serveDownload("", pageData(t, map[string]string{"g": owner, "r": repo, "ref": "HEAD", "path": "../secret"}), nil, nil, nil, 0)
		assert.Nil(t, out)
	})

	t.Run("private repo invisible", func(t *testing.T) {
		out := s.serveDownload("", pageData(t, map[string]string{"g": "user2", "r": "repo2", "ref": "HEAD", "path": "x"}), nil, nil, nil, 0)
		assert.Nil(t, out)
	})
}

// TestServeMediaRequest verifies media path parsing, permission enforcement and
// traversal guards.
func TestServeMediaRequest(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newPageServer(t)

	mediaData := func(t *testing.T, path string) []byte {
		t.Helper()
		body, err := rnsgit.EncodeMixedRequest(map[any]any{"key": ":", "path": path})
		require.NoError(t, err)
		return body
	}

	t.Run("missing key", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{"path": "/media/user2/repo1/HEAD/x"})
		require.NoError(t, err)
		assert.Nil(t, s.serveMediaRequest("", body, nil, nil, nil, 0))
	})

	t.Run("short path", func(t *testing.T) {
		assert.Nil(t, s.serveMediaRequest("", mediaData(t, "/media/user2/repo1"), nil, nil, nil, 0))
	})

	t.Run("private repo invisible", func(t *testing.T) {
		assert.Nil(t, s.serveMediaRequest("", mediaData(t, "/media/user2/repo2/HEAD/x"), nil, nil, nil, 0))
	})

	t.Run("invalid request data", func(t *testing.T) {
		assert.Nil(t, s.serveMediaRequest("", []byte{0xff, 0xff}, nil, nil, nil, 0))
	})
}
