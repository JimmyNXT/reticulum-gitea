// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"testing"
	"time"

	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unittest"
	user_model "gitea.dev/models/user"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepoVars(t *testing.T) {
	for _, tc := range []struct {
		name      string
		vars      map[string]string
		wantOwner string
		wantName  string
		wantOK    bool
	}{
		{"valid", map[string]string{"g": "user2", "r": "repo1"}, "user2", "repo1", true},
		{"missing owner", map[string]string{"r": "repo1"}, "", "", false},
		{"missing name", map[string]string{"g": "user2"}, "", "", false},
		{"slash in owner", map[string]string{"g": "a/b", "r": "repo1"}, "", "", false},
		{"backslash in name", map[string]string{"g": "user2", "r": `a\b`}, "", "", false},
		{"traversal", map[string]string{"g": "user2", "r": ".."}, "", "", false},
		{"trimmed", map[string]string{"g": " user2 ", "r": " repo1 "}, "user2", "repo1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			owner, name, ok := parseRepoVars(tc.vars)
			assert.Equal(t, tc.wantOK, ok)
			assert.Equal(t, tc.wantOwner, owner)
			assert.Equal(t, tc.wantName, name)
		})
	}
}

func TestPageVars(t *testing.T) {
	data, err := encodeMixedRequest(map[any]any{
		"var_g":     "user2",
		"var_r":     "repo1",
		"var_empty": "",
		"other":     "ignored",
	})
	require.NoError(t, err)
	vars := pageVars(data)
	assert.Equal(t, "user2", vars["g"])
	assert.Equal(t, "repo1", vars["r"])
	assert.Equal(t, "", vars["empty"])
	_, has := vars["other"]
	assert.False(t, has)
}

func TestVarInt(t *testing.T) {
	vars := map[string]string{"page": "3", "bad": "x"}
	assert.Equal(t, 3, varInt(vars, "page"))
	assert.Equal(t, 1, varInt(vars, "bad"))
	assert.Equal(t, 1, varInt(vars, "missing"))
}

func TestPageViewerAccess(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := &Server{ctx: t.Context()}

	owner, err := user_model.GetUserByID(t.Context(), 2)
	require.NoError(t, err)

	publicRepo, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo1")
	require.NoError(t, err)
	privateRepo, err := repo_model.GetRepositoryByOwnerAndName(t.Context(), "user2", "repo2")
	require.NoError(t, err)

	anon := viewer{user: nil}
	assert.True(t, s.canViewAs(anon, publicRepo))
	assert.False(t, s.canViewAs(anon, privateRepo))

	signedIn := viewer{user: owner}
	assert.True(t, s.canViewAs(signedIn, publicRepo))
	assert.True(t, s.canViewAs(signedIn, privateRepo))

	blocked := viewer{user: nil, signInReq: true}
	assert.True(t, blocked.blocked())
	assert.False(t, s.canViewAs(blocked, publicRepo))
}

func TestAccessibleRepos(t *testing.T) {
	require.NoError(t, unittest.PrepareTestDatabase())
	s := &Server{ctx: t.Context()}

	owner, err := user_model.GetUserByID(t.Context(), 2)
	require.NoError(t, err)

	anon := s.accessibleRepos(viewer{user: nil, signInReq: false})
	for _, repos := range anon {
		for _, repo := range repos {
			assert.False(t, repo.IsPrivate, "anonymous listing must not expose private repos")
		}
	}

	signedIn := s.accessibleRepos(viewer{user: owner})
	require.Contains(t, signedIn, "user2")
	var names []string
	for _, repo := range signedIn["user2"] {
		names = append(names, repo.Name)
	}
	assert.Contains(t, names, "repo2", "owner must see their private repo")
}

func TestRenderTemplate(t *testing.T) {
	s := &Server{}
	out := string(s.renderTemplate("BODY", "NAV", "front", time.Now()))
	assert.Contains(t, out, "BODY")
	assert.Contains(t, out, "NAV")
	assert.Contains(t, out, "> Repositories")
}

func TestMicronLinkEscaping(t *testing.T) {
	link := mLink("label", pagePathRepo, map[string]string{"g": "a b", "r": "repo"})
	assert.Contains(t, link, "`[label`:")
	assert.Contains(t, link, "g=a+b")

	assert.Equal(t, `\`+"`", mEscape("`"))
	assert.Equal(t, "ab", mEscapeLabel("[a]b"))
}

func TestPrettySize(t *testing.T) {
	assert.Equal(t, "512 B", prettySize(512))
	assert.Equal(t, "1.00 KB", prettySize(1024))
	assert.Equal(t, "1.00 MB", prettySize(1024*1024))
}

func TestNotImplemented(t *testing.T) {
	body := notImplemented("Work pages")
	assert.Contains(t, body, ">Notice")
	assert.Contains(t, body, "Work pages are not available")
}

func TestFormatBlockMarkdown(t *testing.T) {
	out := newMdToMicron().formatBlock("# Title\n\nSome *text*.", "")
	assert.Contains(t, out, ">Title")
	assert.Contains(t, out, "text")
}

func TestMicronFields(t *testing.T) {
	assert.Equal(t, "`", mFields(nil))
	assert.Equal(t, "`g=user2|r=repo1", mFields(map[string]string{"g": "user2", "r": "repo1"}))
}

func TestMicronHeadingsAndFormatting(t *testing.T) {
	assert.Equal(t, ">", mHeading(1))
	assert.Equal(t, ">>>", mHeading(3))
	assert.Equal(t, "`F666dim`f", mDim("dim"))
	assert.Equal(t, "`F0f0text`f", mFg("text", "0f0"))
	assert.Equal(t, "-", mDivider())
	assert.Equal(t, "`!`[bold`:"+pagePathIndex+"]`!", mLinkStrong("bold", pagePathIndex, nil))
}

func TestPrettyTime(t *testing.T) {
	assert.Equal(t, "0 seconds", prettyTime(0))
	assert.Equal(t, "1 minutes", prettyTime(60))
	assert.Equal(t, "2 hours", prettyTime(2*60*60))
	assert.Equal(t, "1 days", prettyTime(24*60*60))
	assert.Equal(t, "never", prettyTimeAgo(0))
	assert.Equal(t, "1970-01-01 00:00:00 UTC", prettyDate(0))
}

func TestFormatTabs(t *testing.T) {
	assert.Equal(t, "a   b", formatTabs("a\tb"))
}

func TestFormatBlockFeatures(t *testing.T) {
	md := newMdToMicron()

	t.Run("bullet list", func(t *testing.T) {
		out := md.formatBlock("- one\n- two", "")
		assert.Contains(t, out, "•")
		assert.Contains(t, out, "one")
		assert.Contains(t, out, "two")
	})

	t.Run("ordered list", func(t *testing.T) {
		out := md.formatBlock("1. first\n2. second", "")
		assert.Contains(t, out, "first")
		assert.Contains(t, out, "second")
	})

	t.Run("blockquote", func(t *testing.T) {
		out := md.formatBlock("> quoted", "")
		assert.Contains(t, out, "quoted")
	})

	t.Run("code fence", func(t *testing.T) {
		out := md.formatBlock("```\ncode line\n```", "")
		assert.Contains(t, out, "code line")
	})

	t.Run("bold and italic", func(t *testing.T) {
		out := md.formatBlock("**bold** and *italic*", "")
		assert.Contains(t, out, "`!bold`!")
		assert.Contains(t, out, "`*italic`*")
	})

	t.Run("link within scope", func(t *testing.T) {
		out := md.formatBlock("[text](page.md)", ":/page/blob.mu`g=u|r=r|path=")
		assert.Contains(t, out, "text")
	})

	t.Run("table", func(t *testing.T) {
		out := md.formatBlock("| a | b |\n|---|---|\n| 1 | 2 |", "")
		assert.Contains(t, out, "a")
		assert.Contains(t, out, "1")
	})
}
