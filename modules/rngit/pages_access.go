// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"encoding/hex"
	"strings"

	asymkey_model "gitea.dev/models/asymkey"
	"gitea.dev/models/db"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"
	"gitea.dev/modules/setting"

	"github.com/Quad4-Software/Reticulum-Go/pkg/identity"
)

const readMode = perm_model.AccessModeRead

// pageUser maps a remote Reticulum identity to its Gitea user, or nil for
// anonymous or unregistered peers.
func (s *Server) pageUser(remote *identity.Identity) *user_model.User {
	if remote == nil {
		return nil
	}
	ident, err := asymkey_model.SearchRNSIdentityByHash(s.ctx, hex.EncodeToString(remote.Hash()))
	if err != nil {
		return nil
	}
	user, _ := user_model.GetUserByID(s.ctx, ident.OwnerID)
	return user
}

// canView reports whether user may read the code unit of repo. Anonymous
// visitors only see public repositories, and only when the instance does not
// require sign-in to view.
func (s *Server) canView(repo *repo_model.Repository, user *user_model.User) bool {
	perm, err := access_model.GetIndividualUserRepoPermission(s.ctx, repo, user)
	if err != nil {
		return false
	}
	return perm.CanAccess(readMode, unit.TypeCode)
}

// viewer bundles the resolved identity of a page request so every handler sees
// the same permission decisions.
type viewer struct {
	user      *user_model.User
	signInReq bool
}

// pageViewer resolves the requester and whether anonymous viewing is allowed.
func (s *Server) pageViewer(remote *identity.Identity) viewer {
	return viewer{
		user:      s.pageUser(remote),
		signInReq: setting.Service.RequireSignInViewStrict,
	}
}

// signedIn reports whether the viewer resolved to a user. When the instance
// requires sign-in, unidentified peers are not served.
func (v viewer) signedIn() bool {
	return v.user != nil
}

// blocked reports whether a request must be refused before doing any work.
func (v viewer) blocked() bool {
	return v.signInReq && !v.signedIn()
}

// canView applies the repo view permission for this viewer.
func (s *Server) canViewAs(v viewer, repo *repo_model.Repository) bool {
	if v.blocked() {
		return false
	}
	return s.canView(repo, v.user)
}

// accessibleRepos lists the repositories the viewer may read, keyed by owner
// name. Anonymous viewers get public repositories only.
func (s *Server) accessibleRepos(v viewer) map[string][]*repo_model.Repository {
	out := map[string][]*repo_model.Repository{}
	if v.blocked() {
		return out
	}
	opts := repo_model.SearchRepoOptions{
		ListOptions: db.ListOptionsAll,
		Actor:       v.user,
		Private:     true,
		OrderBy:     db.SearchOrderByAlphabetically,
	}
	if v.user == nil {
		opts.Private = false
		opts.AllPublic = true
	}
	repos, _, err := repo_model.SearchRepository(s.ctx, opts)
	if err != nil {
		return out
	}
	for _, repo := range repos {
		out[repo.OwnerName] = append(out[repo.OwnerName], repo)
	}
	return out
}

// repoByOwnerName resolves owner/name to a repository, loading the owner so
// permission checks can read its visibility.
func (s *Server) repoByOwnerName(owner, name string) *repo_model.Repository {
	repo, err := repo_model.GetRepositoryByOwnerAndName(s.ctx, owner, name)
	if err != nil {
		return nil
	}
	return repo
}

// parseRepoVars validates the group/repo vars a page request carries, rejecting
// path separators and traversal before they reach lookup.
func parseRepoVars(vars map[string]string) (owner, name string, ok bool) {
	owner = strings.TrimSpace(vars["g"])
	name = strings.TrimSpace(vars["r"])
	if owner == "" || name == "" {
		return "", "", false
	}
	if strings.ContainsAny(owner, "/\\") || strings.ContainsAny(name, "/\\") ||
		strings.Contains(owner, "..") || strings.Contains(name, "..") {
		return "", "", false
	}
	return owner, name, true
}
