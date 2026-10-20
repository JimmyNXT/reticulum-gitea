// Copyright 2026 The Gitea Authors. All rights reserved.
// SPDX-License-Identifier: MIT

package rngit

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitea.dev/modules/git/gitrepo"
	"gitea.dev/modules/log"

	"github.com/Quad4-Software/Reticulum-Go/pkg/identity"
	"github.com/Quad4-Software/Reticulum-Go/pkg/link"
	"github.com/Quad4-Software/Reticulum-Go/pkg/rnsgit"
)

// handleList implements rngit's /git/list (ref advertisement).
func (s *Server) handleList(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	req, repo, user, resp := s.loadRepo(remote, data)
	if resp != nil {
		return resp
	}
	forPush := false
	if v, ok := req["for_push"]; ok {
		if b, ok := v.(bool); ok {
			forPush = b
		}
	}
	if resp := s.checkAccess(repo, user, forPush); resp != nil {
		return resp
	}
	body, err := s.git.ListRefs(gitrepo.RepoLocalPath(repo))
	if err != nil {
		log.Warn("rngit: list refs for %s: %v", repo.FullName(), err)
		return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Could not list refs")
	}
	return append([]byte{rnsgit.ResOK}, []byte(body)...)
}

// handleFetch implements rngit's /git/fetch (bundle generation).
func (s *Server) handleFetch(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	req, repo, user, resp := s.loadRepo(remote, data)
	if resp != nil {
		return resp
	}
	if resp := s.checkAccess(repo, user, false); resp != nil {
		return resp
	}
	refs, names, ok := fetchRefs(req)
	if !ok {
		return rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Invalid request")
	}
	if rnsgit.SanRefs(names) == nil {
		return rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Invalid request")
	}
	var have []string
	if hv, ok := req["have"]; ok {
		if arr, ok := hv.([]any); ok {
			for _, x := range arr {
				if sha := rnsgit.SanSHA(fmt.Sprint(x)); sha != "" {
					have = append(have, sha)
				}
			}
		}
	}
	tmp, err := os.MkdirTemp("", "rnsgit-fetch-")
	if err != nil {
		return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Remote error")
	}
	defer os.RemoveAll(tmp)
	bundlePath := filepath.Join(tmp, "fetch.bundle")
	if err := s.git.CreateBundle(gitrepo.RepoLocalPath(repo), bundlePath, refs, have); err != nil {
		if rnsgit.IsEmptyBundle(err) {
			return []byte{rnsgit.ResOK}
		}
		log.Warn("rngit: create bundle for %s: %v", repo.FullName(), err)
		return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Could not fetch refs")
	}
	bundle, err := os.ReadFile(bundlePath)
	if err != nil {
		return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Remote error")
	}
	return link.FileResponse{Data: bundle, MetadataPacked: rnsgit.OKMetadataPacked(), AutoCompress: true}
}

// fetchRefs normalizes the refs array of a fetch request.
func fetchRefs(req map[any]any) ([]map[string]string, []string, bool) {
	refsAny, ok := req["refs"]
	if !ok {
		return nil, nil, false
	}
	refsList, ok := refsAny.([]any)
	if !ok {
		return nil, nil, false
	}
	refs := make([]map[string]string, 0, len(refsList))
	names := make([]string, 0, len(refsList))
	for _, item := range refsList {
		m, ok := item.(map[any]any)
		if !ok {
			m2, ok := item.(map[string]any)
			if !ok {
				return nil, nil, false
			}
			m = map[any]any{}
			for k, v := range m2 {
				m[k] = v
			}
		}
		ref := fmt.Sprint(m["ref"])
		if rnsgit.SanRef(ref) == "" {
			return nil, nil, false
		}
		entry := map[string]string{"ref": ref}
		if hv, ok := m["have"]; ok {
			entry["have"] = fmt.Sprint(hv)
		}
		refs = append(refs, entry)
		names = append(names, ref)
	}
	return refs, names, true
}

// handlePush implements rngit's /git/push (bundle apply or ref update).
func (s *Server) handlePush(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	req, repo, user, resp := s.loadRepo(remote, data)
	if resp != nil {
		return resp
	}
	if resp := s.checkAccess(repo, user, true); resp != nil {
		return resp
	}
	repoPath := gitrepo.RepoLocalPath(repo)
	before := s.snapshotRefs(repoPath)

	if bundle, ok := req["bundle"]; ok {
		localRef := rnsgit.SanRef(fmt.Sprint(req["local_ref"]))
		remoteRef := rnsgit.SanRef(fmt.Sprint(req["remote_ref"]))
		force, _ := req["force"].(bool)
		if localRef == "" || remoteRef == "" {
			return rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Missing ref specification")
		}
		var bundleData []byte
		switch b := bundle.(type) {
		case []byte:
			bundleData = b
		case string:
			bundleData = []byte(b)
		default:
			return rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Invalid bundle")
		}
		tmp, err := os.MkdirTemp("", "rnsgit-push-")
		if err != nil {
			return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Remote error")
		}
		defer os.RemoveAll(tmp)
		bundlePath := filepath.Join(tmp, "push.bundle")
		if err := os.WriteFile(bundlePath, bundleData, 0o600); err != nil {
			return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Remote error")
		}
		if err := s.git.VerifyBundle(repoPath, bundlePath); err != nil {
			return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Could not verify bundle")
		}
		if err := s.git.FetchBundle(repoPath, bundlePath, localRef, remoteRef, force); err != nil {
			return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Could not verify bundle")
		}
		s.postReceive(repo, user, before)
		return []byte{rnsgit.ResOK}
	}

	if ops, ok := req["operations"]; ok {
		list, ok := ops.([]any)
		if !ok {
			return rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Invalid data for operations")
		}
		for _, item := range list {
			m, ok := item.(map[any]any)
			if !ok {
				m2, ok := item.(map[string]any)
				if !ok {
					return rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Invalid request")
				}
				m = map[any]any{}
				for k, v := range m2 {
					m[k] = v
				}
			}
			if fmt.Sprint(m["action"]) != "update_ref" {
				return rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Unknown operation")
			}
			ref := rnsgit.SanRef(fmt.Sprint(m["ref"]))
			sha := rnsgit.SanSHA(fmt.Sprint(m["sha"]))
			force, _ := m["force"].(bool)
			if ref == "" || sha == "" || !strings.HasPrefix(ref, "refs/") {
				return rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Invalid request")
			}
			if !s.git.ObjectExists(repoPath, sha) {
				return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Object does not exist in repository")
			}
			if err := s.git.UpdateRef(repoPath, ref, sha, force); err != nil {
				if strings.Contains(err.Error(), "different sha") {
					return rnsgit.StatusResponse(rnsgit.ResDisallowed, "Ref exists at different SHA (force required)")
				}
				return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Could not update refs")
			}
		}
		s.postReceive(repo, user, before)
		return []byte{rnsgit.ResOK}
	}
	return rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Invalid request data")
}

// handleDelete implements rngit's /git/delete (ref deletion).
func (s *Server) handleDelete(_ string, data, _, _ []byte, remote *identity.Identity, _ int64) any {
	req, repo, user, resp := s.loadRepo(remote, data)
	if resp != nil {
		return resp
	}
	if resp := s.checkAccess(repo, user, true); resp != nil {
		return resp
	}
	ref := rnsgit.SanRef(fmt.Sprint(req["ref"]))
	if ref == "" || !strings.HasPrefix(ref, "refs/") {
		return rnsgit.StatusResponse(rnsgit.ResInvalidReq, "Invalid request")
	}
	repoPath := gitrepo.RepoLocalPath(repo)
	before := s.snapshotRefs(repoPath)
	if err := s.git.DeleteRef(repoPath, ref); err != nil {
		return rnsgit.StatusResponse(rnsgit.ResRemoteFail, "Could not delete ref")
	}
	s.postReceive(repo, user, before)
	return []byte{rnsgit.ResOK}
}
