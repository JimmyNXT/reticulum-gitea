// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"encoding/hex"

	asymkey_model "gitea.dev/models/asymkey"
	perm_model "gitea.dev/models/perm"
	access_model "gitea.dev/models/perm/access"
	repo_model "gitea.dev/models/repo"
	"gitea.dev/models/unit"
	user_model "gitea.dev/models/user"

	"github.com/Quad4-Software/Reticulum-Go/pkg/identity"
	"github.com/Quad4-Software/Reticulum-Go/pkg/rnsgit"
)

// loadRepo decodes an rngit request and resolves the target repository, mapping
// the remote Reticulum identity to a Gitea user when it is registered.
func (s *Server) loadRepo(remote *identity.Identity, data []byte) (map[any]any, *repo_model.Repository, *user_model.User, []byte) {
	req, err := rnsgit.DecodeRequest(data)
	if err != nil {
		return nil, nil, nil, rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Invalid request")
	}
	repoPath, ok := rnsgit.RepoFromRequest(req)
	if !ok {
		return nil, nil, nil, rnsgit.StatusResponse(rnsgit.ResInvalidReq, "No repository specified")
	}
	owner, name, ok := rnsgit.ParseRepoPath(repoPath)
	if !ok {
		return nil, nil, nil, rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Invalid request")
	}
	repo, err := repo_model.GetRepositoryByOwnerAndName(s.ctx, owner, name)
	if err != nil {
		return nil, nil, nil, rnsgit.StatusResponse(rnsgit.ResNotFound, "Not found")
	}
	var user *user_model.User
	if remote != nil {
		if ident, err := asymkey_model.SearchRNSIdentityByHash(s.ctx, hex.EncodeToString(remote.Hash())); err == nil {
			user, _ = user_model.GetUserByID(s.ctx, ident.OwnerID)
		}
	}
	return req, repo, user, nil
}

// checkAccess returns a non-nil rngit status response when user may not access
// the code unit of repo at the requested level. An unregistered or anonymous
// remote still reads public repositories.
func (s *Server) checkAccess(repo *repo_model.Repository, user *user_model.User, write bool) []byte {
	perm, err := access_model.GetIndividualUserRepoPermission(s.ctx, repo, user)
	if err != nil {
		return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Remote error")
	}
	mode := perm_model.AccessModeRead
	if write {
		mode = perm_model.AccessModeWrite
	}
	if perm.CanAccess(mode, unit.TypeCode) {
		return nil
	}
	if perm.CanAccess(perm_model.AccessModeRead, unit.TypeCode) {
		return rnsgit.StatusResponse(rnsgit.ResDisallowed, "Not allowed")
	}
	return rnsgit.StatusResponse(rnsgit.ResNotFound, "Not found")
}
