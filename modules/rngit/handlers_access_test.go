// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"testing"

	"gitea.dev/models/unittest"
	"gitea.dev/modules/git/gitcmd"

	"github.com/Quad4-Software/Reticulum-Go/pkg/rnsgit"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestServer builds a Server wired to the real git executable, without a
// Reticulum node, for exercising the request handlers directly.
func newTestServer(t *testing.T) *Server {
	t.Helper()
	return &Server{ctx: t.Context(), git: &rnsgit.GitRunner{Git: gitcmd.GitExecutable}}
}

// TestLoadRepo covers request decoding, repository resolution and identity to
// user mapping in loadRepo.
func TestLoadRepo(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newTestServer(t)

	reqBody, err := rnsgit.EncodeMixedRequest(map[any]any{rnsgit.IdxRepository: "user2/repo1"})
	require.NoError(t, err)

	t.Run("valid public repo anonymous", func(t *testing.T) {
		req, repo, user, resp := s.loadRepo(nil, reqBody)
		assert.Nil(t, resp)
		require.NotNil(t, repo)
		assert.Equal(t, "repo1", repo.Name)
		assert.Nil(t, user)
		assert.NotNil(t, req)
	})

	t.Run("invalid request data", func(t *testing.T) {
		_, _, _, resp := s.loadRepo(nil, []byte{0xff, 0xff, 0xff})
		require.NotNil(t, resp)
		assert.Equal(t, byte(rnsgit.ResInvalidReq), resp[0])
	})

	t.Run("no repository specified", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{})
		require.NoError(t, err)
		_, _, _, resp := s.loadRepo(nil, body)
		require.NotNil(t, resp)
		assert.Equal(t, byte(rnsgit.ResInvalidReq), resp[0])
	})

	t.Run("unknown repo is not found", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{rnsgit.IdxRepository: "user2/does-not-exist"})
		require.NoError(t, err)
		_, _, _, resp := s.loadRepo(nil, body)
		require.NotNil(t, resp)
		assert.Equal(t, byte(rnsgit.ResNotFound), resp[0])
	})
}

// TestHandleList covers the ref advertisement path including permission
// enforcement and for_push handling.
func TestHandleList(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newTestServer(t)

	publicBody, err := rnsgit.EncodeMixedRequest(map[any]any{rnsgit.IdxRepository: "user2/repo1"})
	require.NoError(t, err)
	privateBody, err := rnsgit.EncodeMixedRequest(map[any]any{rnsgit.IdxRepository: "user2/repo2"})
	require.NoError(t, err)

	t.Run("list public anonymous", func(t *testing.T) {
		body, ok := s.handleList("", publicBody, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		require.NotEmpty(t, body)
		assert.Equal(t, byte(rnsgit.ResOK), body[0])
	})

	t.Run("list private anonymous not found", func(t *testing.T) {
		body, ok := s.handleList("", privateBody, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		assert.Equal(t, byte(rnsgit.ResNotFound), body[0])
	})

	t.Run("list with push flag on public is disallowed", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{
			rnsgit.IdxRepository: "user2/repo1",
			"for_push":           true,
		})
		require.NoError(t, err)
		resp, ok := s.handleList("", body, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		assert.Equal(t, byte(rnsgit.ResDisallowed), resp[0])
	})
}

// TestHandleFetch validates request parsing and ref sanitisation before any git
// work happens.
func TestHandleFetch(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newTestServer(t)

	t.Run("missing refs", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{rnsgit.IdxRepository: "user2/repo1"})
		require.NoError(t, err)
		resp, ok := s.handleFetch("", body, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		assert.Equal(t, byte(rnsgit.ResInvalidReq), resp[0])
	})

	t.Run("invalid ref ignored", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{
			rnsgit.IdxRepository: "user2/repo1",
			"refs":               []any{map[any]any{"ref": "bad ref"}},
		})
		require.NoError(t, err)
		resp, ok := s.handleFetch("", body, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		assert.Equal(t, byte(rnsgit.ResInvalidReq), resp[0])
	})

	t.Run("private repo anonymous is not found", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{
			rnsgit.IdxRepository: "user2/repo2",
			"refs":               []any{map[any]any{"ref": "refs/heads/main"}},
		})
		require.NoError(t, err)
		resp, ok := s.handleFetch("", body, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		assert.Equal(t, byte(rnsgit.ResNotFound), resp[0])
	})
}

// TestHandleDelete validates ref deletion guards: access, ref shape and
// permission on private repositories.
func TestHandleDelete(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newTestServer(t)

	t.Run("missing ref anonymous is disallowed", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{rnsgit.IdxRepository: "user2/repo1"})
		require.NoError(t, err)
		resp, ok := s.handleDelete("", body, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		assert.Equal(t, byte(rnsgit.ResDisallowed), resp[0])
	})

	t.Run("ref without refs/ prefix anonymous is disallowed", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{
			rnsgit.IdxRepository: "user2/repo1",
			"ref":                "main",
		})
		require.NoError(t, err)
		resp, ok := s.handleDelete("", body, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		assert.Equal(t, byte(rnsgit.ResDisallowed), resp[0])
	})

	t.Run("private repo anonymous is not found", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{
			rnsgit.IdxRepository: "user2/repo2",
			"ref":                "refs/heads/main",
		})
		require.NoError(t, err)
		resp, ok := s.handleDelete("", body, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		assert.Equal(t, byte(rnsgit.ResNotFound), resp[0])
	})
}

// TestHandlePush validates the write-side guards that run before any ref is
// touched.
func TestHandlePush(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := newTestServer(t)

	t.Run("anonymous push public is disallowed", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{
			rnsgit.IdxRepository: "user2/repo1",
			"operations":         []any{map[any]any{"action": "update_ref", "ref": "refs/heads/x", "sha": "0123456789012345678901234567890123456789"}},
		})
		require.NoError(t, err)
		resp, ok := s.handlePush("", body, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		assert.Equal(t, byte(rnsgit.ResDisallowed), resp[0])
	})

	t.Run("private repo anonymous is not found", func(t *testing.T) {
		body, err := rnsgit.EncodeMixedRequest(map[any]any{rnsgit.IdxRepository: "user2/repo2"})
		require.NoError(t, err)
		resp, ok := s.handlePush("", body, nil, nil, nil, 0).([]byte)
		require.True(t, ok)
		assert.Equal(t, byte(rnsgit.ResNotFound), resp[0])
	})
}
