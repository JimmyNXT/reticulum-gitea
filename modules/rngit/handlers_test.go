// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFetchRefs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		req       map[any]any
		wantNames []string
		wantOK    bool
	}{
		{"missing refs", map[any]any{}, nil, false},
		{"refs not a list", map[any]any{"refs": "refs/heads/main"}, nil, false},
		{"invalid ref", map[any]any{"refs": []any{map[any]any{"ref": "bad ref"}}}, nil, false},
		{"not a map", map[any]any{"refs": []any{"refs/heads/main"}}, nil, false},
		{
			"valid",
			map[any]any{"refs": []any{
				map[any]any{"ref": "refs/heads/main"},
				map[any]any{"ref": "refs/heads/dev", "have": "0123456789012345678901234567890123456789"},
			}},
			[]string{"refs/heads/main", "refs/heads/dev"},
			true,
		},
		{
			"string-keyed map",
			map[any]any{"refs": []any{map[string]any{"ref": "refs/tags/v1"}}},
			[]string{"refs/tags/v1"},
			true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			refs, names, ok := fetchRefs(tc.req)
			assert.Equal(t, tc.wantOK, ok)
			if !tc.wantOK {
				return
			}
			assert.Equal(t, tc.wantNames, names)
			assert.Len(t, refs, len(tc.wantNames))
		})
	}
}
